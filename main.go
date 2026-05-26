package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// ── Configuration ──────────────────────────────────────────────────────────────

const (
	// URL pattern for each medal ID on the GamersClub CDN.
	baseURL = "https://gcv1-assets.gamersclub.com.br/images/medalhas/%d.png"

	// Local directory where medals are saved.
	outputDir = "medalhas"

	// Scanning always starts from this ID.
	startID = 0

	// Concurrency & rate-limit settings.
	// Workers slightly exceeds rate so the limiter — not goroutine scheduling —
	// is always the pacing mechanism.
	maxWorkers     = 30
	requestsPerSec = 25
	burstSize      = 50 // initial burst to fill the HTTP/2 pipeline fast

	// Retry policy.
	maxRetries     = 3
	retryBaseDelay = 500 * time.Millisecond

	// Per-request timeout (download). Discovery uses half this value.
	requestTimeout = 15 * time.Second

	// Upper-bound discovery: safety cap (avoids infinite probing).
	discoveryCap = 1_000_000
)

// copyBufPool holds reusable 64 KB I/O buffers, one per concurrent download.
// Eliminates per-request heap allocations and reduces GC pressure.
var copyBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 64*1024)
		return &buf
	},
}

// ── Counters ───────────────────────────────────────────────────────────────────

var (
	cntDownloaded atomic.Int64
	cntSkipped    atomic.Int64
	cntFailed     atomic.Int64
)

// ── Upper-bound discovery ──────────────────────────────────────────────────────

// probeExists makes a lightweight HEAD request to check whether a medal ID
// is served by the CDN. Used only during the discovery phase.
func probeExists(ctx context.Context, client *http.Client, id int) bool {
	url := fmt.Sprintf(baseURL, id)
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout/2)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodHead, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// discoverUpperBound returns the highest medal ID currently served by the CDN.
//
// Algorithm — O(log N) HEAD requests:
//  1. Exponential probe: double the candidate bound until the CDN returns 404,
//     establishing a bracket [lo, hi] that straddles the true maximum.
//  2. Binary search within that bracket to find the exact last valid ID.
//
// This replaces any hardcoded upper bound, making the tool self-adapting as
// GamersClub adds new medals over time.
func discoverUpperBound(ctx context.Context, client *http.Client) int {
	log.Printf("[DISCOVERY] probing CDN upper bound (O(log N) HEAD requests)…")

	// Phase 1: exponential growth to bracket the upper end.
	lo, hi := 0, 100
	for probeExists(ctx, client, hi) {
		lo = hi
		hi *= 2
		if hi > discoveryCap {
			log.Printf("[DISCOVERY] hit safety cap at %d", discoveryCap)
			return discoveryCap
		}
	}

	// Phase 2: binary search within [lo, hi].
	for lo < hi-1 {
		mid := (lo + hi) / 2
		if probeExists(ctx, client, mid) {
			lo = mid
		} else {
			hi = mid
		}
	}

	log.Printf("[DISCOVERY] upper bound = %d  (~%d HEAD requests)", lo, 2*int(math.Log2(float64(lo+1))+1))
	return lo
}

// ── Download logic ─────────────────────────────────────────────────────────────

func exponentialDelay(attempt int) time.Duration {
	return time.Duration(math.Pow(2, float64(attempt-1))) * retryBaseDelay
}

// httpGet performs a single GET request for url.
// On HTTP 200 it streams the body directly to dst; on any other status it
// returns (statusCode, nil). Network errors return (0, err).
//
// The per-request context lives for the entire call — including the body read —
// so cancel() is deferred to function return, never called before io.Copy.
func httpGet(ctx context.Context, client *http.Client, url, dst string) (int, error) {
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel() // kept alive until body is fully read

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; gamersclub-medal-catcher/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}

	f, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}
	bufp := copyBufPool.Get().(*[]byte)
	_, copyErr := io.CopyBuffer(f, resp.Body, *bufp)
	copyBufPool.Put(bufp)
	f.Close()
	if copyErr != nil {
		os.Remove(dst)
		return 0, copyErr
	}
	return http.StatusOK, nil
}

// downloadMedal downloads a single medal, retrying on transient failures.
func downloadMedal(ctx context.Context, client *http.Client, limiter *rate.Limiter, id int) error {
	dst := filepath.Join(outputDir, fmt.Sprintf("medal_%d.png", id))
	url := fmt.Sprintf(baseURL, id)
	var lastErr error

	for a := 1; a <= maxRetries; a++ {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}

		code, err := httpGet(ctx, client, url, dst)

		if err != nil {
			lastErr = err
			if a < maxRetries {
				d := exponentialDelay(a)
				log.Printf("[WARN] id=%-4d attempt=%d — %v  (retry in %v)", id, a, err, d)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(d):
				}
			}
			continue
		}

		switch {
		case code == http.StatusOK:
			cntDownloaded.Add(1)
			log.Printf("[OK]   medal_%d.png", id)
			return nil
		case code == http.StatusNotFound:
			cntSkipped.Add(1)
			return nil
		case code == http.StatusTooManyRequests || code >= 500:
			lastErr = fmt.Errorf("HTTP %d", code)
			if a < maxRetries {
				d := exponentialDelay(a)
				log.Printf("[WARN] id=%-4d HTTP %d  (retry in %v)", id, code, d)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(d):
				}
			}
			continue
		default:
			cntSkipped.Add(1)
			return nil
		}
	}

	cntFailed.Add(1)
	return fmt.Errorf("all %d attempts failed (id=%d): %w", maxRetries, id, lastErr)
}

// ── Entry point ────────────────────────────────────────────────────────────────

// loadExisting scans outputDir and returns a set of already-downloaded IDs.
// Using a pre-built map is O(1) per lookup and avoids a syscall per ID.
func loadExisting() map[int]struct{} {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil
	}
	existing := make(map[int]struct{}, len(entries))
	for _, e := range entries {
		var id int
		if n, _ := fmt.Sscanf(e.Name(), "medal_%d.png", &id); n == 1 {
			existing[id] = struct{}{}
		}
	}
	return existing
}

// handle is the single public function of this tool.
// It auto-discovers the current medal range from the CDN and downloads every
// available medal, skipping ones already present on disk.
func handle() {
	log.SetFlags(log.Ltime)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Fatalf("create output dir %q: %v", outputDir, err)
	}

	ctx := context.Background()

	// HTTP client with HTTP/2 enabled and explicit transport timeouts.
	// ForceAttemptHTTP2 enables multiplexing: multiple requests share a single
	// TCP+TLS connection, eliminating per-request handshake overhead.
	client := &http.Client{
		Transport: &http.Transport{
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          maxWorkers * 2,
			MaxIdleConnsPerHost:   maxWorkers,
			MaxConnsPerHost:       maxWorkers,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}

	// Auto-discover the upper bound — no hardcoded limit.
	upperBound := discoverUpperBound(ctx, client)

	// Pre-load already-downloaded files: O(1) map lookup replaces per-request os.Stat.
	existing := loadExisting()
	cntDownloaded.Add(int64(len(existing)))
	if len(existing) > 0 {
		log.Printf("[CACHE] %d medals already on disk, skipping", len(existing))
	}

	// Token-bucket limiter: fill rate + generous burst to saturate the HTTP/2 pipeline.
	limiter := rate.NewLimiter(rate.Limit(requestsPerSec), burstSize)

	// Feed only IDs not yet on disk into the work channel.
	total := upperBound - startID + 1
	ids := make(chan int, total)
	needDownload := 0
	for i := startID; i <= upperBound; i++ {
		if _, ok := existing[i]; !ok {
			ids <- i
			needDownload++
		}
	}
	close(ids)

	log.Printf("Downloading | range [%d, %d] | to fetch=%d | workers=%d | rate=%d req/s | burst=%d",
		startID, upperBound, needDownload, maxWorkers, requestsPerSec, burstSize)

	start := time.Now()
	var wg sync.WaitGroup

	for range maxWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range ids {
				if err := downloadMedal(ctx, client, limiter, id); err != nil {
					log.Printf("[ERR]  id=%d: %v", id, err)
				}
			}
		}()
	}

	wg.Wait()

	fmt.Printf("\n════════════════════════════════\n")
	fmt.Printf("  Finished in %v\n", time.Since(start).Round(time.Millisecond))
	fmt.Printf("  Downloaded : %d\n", cntDownloaded.Load())
	fmt.Printf("  Skipped    : %d\n", cntSkipped.Load())
	fmt.Printf("  Failed     : %d\n", cntFailed.Load())
	fmt.Printf("════════════════════════════════\n")
}

func main() {
	handle()
}
