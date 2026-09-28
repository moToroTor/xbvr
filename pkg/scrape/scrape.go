package scrape

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
	"github.com/xbapps/xbvr/pkg/common"
	"github.com/xbapps/xbvr/pkg/config"
	"github.com/xbapps/xbvr/pkg/models"
	"golang.org/x/net/html"
)

var log = &common.Log

// UserAgent is sent by every scraper and by the HTML/JSON trailer-source scrapes.
//
// It must not be left on a string that studios have blocklisted. The previous value,
// "Chrome/73.0.3683.103", is rejected outright (HTTP 403) by realjamvr.com and porncornvr.com,
// which silently breaks both scrapers: every scheduled run fails with
// "Error visiting https://<site>/scenes Forbidden" and no scenes are ingested.
//
// The block is on the EXACT literal version string, not on "old browser" heuristics. Verified
// live: "Chrome/73.0.3683.104" (one digit different) and "Chrome/73.0.0.0" both return 200 from
// the same host that 403s "Chrome/73.0.3683.103". Because that exact string is XBVR's shipped
// default, every install sends it verbatim, making it a reliable fingerprint for the client -
// which is precisely what makes it worth blocklisting. Keep this a realistic, current browser UA;
// if a studio starts 403ing again, check for a blocklisted-fingerprint 403 before assuming the
// site itself changed.
var UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36"

func createCollector(domains ...string) *colly.Collector {
	// Also allow the www./naked counterpart of each domain: colly's
	// AllowedDomains is an exact host match, so a single-scene URL pasted
	// with a host variant the scraper didn't register (e.g. a www. prefix
	// on https://www.realitylovers.com/...) was rejected with a silent
	// ErrForbiddenDomain and the scrape finished with 0 scenes and no error.
	// NOTE: several scrapers still pass explicit "www." entries alongside
	// (or instead of) the bare domain; those are now redundant and could be
	// removed in a cleanup, but are left untouched to keep this change minimal.
	allowed := make([]string, 0, len(domains)*2)
	for _, d := range domains {
		allowed = append(allowed, d)
		if strings.HasPrefix(d, "www.") {
			allowed = append(allowed, strings.TrimPrefix(d, "www."))
		} else {
			allowed = append(allowed, "www."+d)
		}
	}
	c := colly.NewCollector(
		colly.AllowedDomains(allowed...),
		colly.CacheDir(getScrapeCacheDir()),
		colly.UserAgent(UserAgent),
	)
	// use proxy if configured
	if config.Config.Advanced.ScraperProxy != "" {
		common.Log.Infof("Using proxy for scraping: %s.", config.Config.Advanced.ScraperProxy)
		c.SetProxy(config.Config.Advanced.ScraperProxy)
	}

	// Set error handler
	c.OnError(func(r *colly.Response, err error) {
		log.Errorf("Error visiting %s %s", r.Request.URL, err)
	})

	c = createCallbacks(c)

	// see if the domain has a limit and set it
	for _, domain := range domains {
		SetupCollector(GetCoreDomain(domain)+"-scraper", c)
		log.Debugf("Using Header/Cookies from %s", GetCoreDomain(domain)+"-scraper")
		if Limiters == nil {
			LoadScraperRateLimits()
		}
		limiter := GetRateLimiter(domain)
		if limiter != nil {
			randomdelay := limiter.maxDelay - limiter.minDelay
			delay := limiter.minDelay
			c.Limit(&colly.LimitRule{
				DomainGlob:  "*",
				Delay:       delay,       // Delay between requests to domains matching the glob
				RandomDelay: randomdelay, // Max additional random delay added to the delay
			})
			break
		}
	}

	return c
}

func cloneCollector(c *colly.Collector) *colly.Collector {
	x := c.Clone()
	x = createCallbacks(x)
	return x
}

// allowURLRevisit opts a single collector out of Colly's URL-visit dedup.
//
// Colly records every redirect target in the shared visited set, so a
// transient geo-gate redirect (e.g. a FuckPassVR scene 302 to /sfw/)
// poisons dedup: the first redirect marks /sfw/ visited and every later
// redirected scene fails with "already visited" (xbvr#2160). Callers that
// opt out must dedup the canonical URLs they actually want themselves
// (e.g. a local map). Default createCollector behaviour is unchanged for
// all other scrapers.
func allowURLRevisit(c *colly.Collector) *colly.Collector {
	c.AllowURLRevisit = true
	return c
}

func createCallbacks(c *colly.Collector) *colly.Collector {
	const maxRetries = 15
	const maxTransientRetries = 3

	c.OnRequest(func(r *colly.Request) {
		attempt := r.Ctx.GetAny("attempt")

		if attempt == nil {
			r.Ctx.Put("attempt", 1)
		}

		log.Infoln("visiting", r.URL.String())
	})

	c.OnError(func(r *colly.Response, err error) {
		attempt := r.Ctx.GetAny("attempt").(int)

		retried := false
		if r.StatusCode == 429 {
			log.Errorln("Error:", r.StatusCode, err)

			if attempt <= maxRetries {
				unCache(r.Request.URL.String(), c.CacheDir)
				delay := retryDelay(attempt)
				log.Errorf("Waiting %s before next request (attempt %d)...", delay, attempt)
				r.Ctx.Put("attempt", attempt+1)
				retrySleep(delay)
				r.Request.Retry()
				retried = true
			}
		} else if attempt <= maxTransientRetries && isTransientVisitError(r.StatusCode, err) {
			// Transient failures (timeouts, 5xx) get a small retry budget so
			// a single dead page doesn't silently drop scenes (a failed
			// listing page is never revisited otherwise). 403/404 and other
			// 4xx are permanent: retrying a block must not mask the block
			// signal, and a missing page won't appear on retry.
			unCache(r.Request.URL.String(), c.CacheDir)
			delay := retryDelay(attempt)
			log.Errorf("Transient error visiting %s (%s), retrying in %s (%d/%d)",
				r.Request.URL, err, delay, attempt, maxTransientRetries)
			r.Ctx.Put("attempt", attempt+1)
			retrySleep(delay)
			r.Request.Retry()
			retried = true
		}
		if !retried {
			recordVisitError(r.Request.URL.Hostname(), r.StatusCode)
		}
	})

	return c
}

func DeleteScrapeCache() error {
	return os.RemoveAll(getScrapeCacheDir())
}

func getScrapeCacheDir() string {
	return common.ScrapeCacheDir
}

func registerScraper(id string, name string, avatarURL string, domain string, f models.ScraperFunc) {
	models.RegisterScraper(id, name, avatarURL, domain, f, "")
}

func registerAlternateScraper(id string, name string, avatarURL string, domain string, masterSiteId string, f models.ScraperFunc) {
	// alternate scrapers are to scrape scenes available at other sites to match against a scenes from the studio's site, eg scrape VRHush scenes from SLR and match to scenes from VRHush
	models.RegisterScraper(id, name, avatarURL, domain, f, masterSiteId)
}

func logScrapeStart(id string, name string) {
	log.WithFields(logrus.Fields{
		"task":      "scraperProgress",
		"scraperID": id,
		"progress":  0,
		"started":   true,
		"completed": false,
	}).Infof("Starting %v scraper", name)
}

func logScrapeFinished(id string, name string) {
	log.WithFields(logrus.Fields{
		"task":      "scraperProgress",
		"scraperID": id,
		"progress":  0,
		"started":   false,
		"completed": true,
	}).Infof("Finished %v scraper", name)
}

func unCache(URL string, cacheDir string) {
	sum := sha1.Sum([]byte(URL))
	hash := hex.EncodeToString(sum[:])
	dir := path.Join(cacheDir, hash[:2])
	filename := path.Join(dir, hash)
	// A failed visit may never have been cached (e.g. timeouts); only a
	// real removal failure is fatal.
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
}

// retrySleep waits between attempts; a variable so tests can stub it out.
var retrySleep = time.Sleep

// Per-scraper counts of terminal page failures, for the run status shown
// on the Scrapers page ("blocked" vs "done"). Fed from OnError, drained
// by the task runner after each scraper finishes (TakeRunStats).
type visitStats struct {
	blocked int // HTTP 403: the site is refusing us
	failed  int // everything else terminal: 404s, exhausted retries, ...
}

var runStats = struct {
	sync.Mutex
	m map[string]*visitStats
}{m: map[string]*visitStats{}}

var scraperDomainMap struct {
	once sync.Once
	m    map[string][]string
}

func domainScraperMap() map[string][]string {
	scraperDomainMap.once.Do(func() {
		m := map[string][]string{}
		for _, s := range models.GetScrapers() {
			if s.Domain == "" {
				continue
			}
			d := GetCoreDomain(strings.ToLower(s.Domain))
			m[d] = append(m[d], s.ID)
		}
		scraperDomainMap.m = m
	})
	return scraperDomainMap.m
}

// scraperIDsForHost resolves a request host to the scrapers serving it via
// their registered core domains. Shared infrastructure (e.g. povr.com
// serving tranzvr, brasilvr, ...) attributes to every scraper on it:
// when the shared host refuses, all of them are blocked.
func scraperIDsForHost(scrapers []models.Scraper, host string) []string {
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	want := GetCoreDomain(strings.ToLower(host))
	var ids []string
	for _, s := range scrapers {
		if s.Domain == "" {
			continue
		}
		if GetCoreDomain(strings.ToLower(s.Domain)) == want {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

func recordVisitError(host string, statusCode int) {
	ids := domainScraperMap()[GetCoreDomain(strings.ToLower(host))]
	if len(ids) == 0 {
		return
	}
	blocked := statusCode == 403
	runStats.Lock()
	defer runStats.Unlock()
	for _, id := range ids {
		st := runStats.m[id]
		if st == nil {
			st = &visitStats{}
			runStats.m[id] = st
		}
		if blocked {
			st.blocked++
		} else {
			st.failed++
		}
	}
}

// TakeRunStats returns and clears the failure counters for one scraper.
func TakeRunStats(scraperID string) (blocked, failed int) {
	runStats.Lock()
	defer runStats.Unlock()
	st := runStats.m[scraperID]
	if st == nil {
		return 0, 0
	}
	delete(runStats.m, scraperID)
	return st.blocked, st.failed
}

// retryDelay backs off exponentially from 2s, capped at 30s, so repeated
// failures (a rate limiter answering 429 sixteen times in a row) slow
// down instead of hammering on a fixed 2s metronome.
func retryDelay(attempt int) time.Duration {
	d := 2 * time.Second
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= 30*time.Second {
			return 30 * time.Second
		}
	}
	return d
}

// isTransientVisitError reports whether a failed page visit is worth
// retrying: timeouts, reset connections and 5xx. Anything else —
// notably 403 (blocked) and 404 (gone) — is permanent.
func isTransientVisitError(statusCode int, err error) bool {
	if statusCode >= 500 && statusCode <= 599 {
		return true
	}
	if statusCode != 0 {
		return false
	}
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := err.Error()
	for _, s := range []string{
		"Client.Timeout",
		"context deadline exceeded",
		"connection reset by peer",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

func updateSiteLastUpdate(id string) {
	var site models.Site
	err := site.GetIfExist(id)
	if err != nil {
		log.Error(err)
		return
	}
	site.LastUpdate = time.Now()
	site.Save()
}

func traverseNodes(node *html.Node, fn func(*html.Node)) {
	if node == nil {
		return
	}

	fn(node)

	for cur := node.FirstChild; cur != nil; cur = cur.NextSibling {
		traverseNodes(cur, fn)
	}
}

func findComments(sel *goquery.Selection) []string {
	comments := []string{}
	for _, node := range sel.Nodes {
		traverseNodes(node, func(node *html.Node) {
			if node.Type == html.CommentNode {
				comments = append(comments, node.Data)
			}
		})
	}
	return comments
}

func getFilenameFromURL(u string) string {
	p, _ := url.Parse(u)
	return path.Base(p.Path)
}

func getTextFromHTMLWithSelector(data string, sel string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(doc.Find(sel).Text())
}
func CreateCollector(domains ...string) *colly.Collector {
	return createCollector(domains...)
}

func GetCoreDomain(domain string) string {
	if strings.HasPrefix(domain, "http") {
		parsedURL, _ := url.Parse(domain)
		domain = parsedURL.Hostname()
	}
	parts := strings.Split(domain, ".")
	if len(parts) > 2 && parts[0] == "www" {
		parts = parts[1:]
	}

	return strings.Join(parts[:len(parts)-1], ".")
}
