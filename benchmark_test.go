package rcutil_test

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/k1LoW/rc"
	"github.com/k1LoW/rcutil/testutil"
	"github.com/k1LoW/rp"
	testur "github.com/k1LoW/rp/testutil"
	"golang.org/x/sync/errgroup"
)

func BenchmarkNGINXCache(b *testing.B) {
	hostname := "a.example.com"
	_ = testutil.NewUpstreamEchoNGINXServer(b, hostname)
	upstreams := map[string]string{}
	upstreams[hostname] = fmt.Sprintf("http://%s:80", hostname)
	proxy := testutil.NewReverseProxyNGINXServer(b, "r.example.com", upstreams)

	// Make cache
	const (
		concurrency = 1
		cacherange  = 10000
	)
	limitCh := make(chan struct{}, concurrency)
	eg := new(errgroup.Group)
	for i := 0; i < cacherange; i++ {
		i := i
		limitCh <- struct{}{}
		eg.Go(func() error {
			defer func() {
				<-limitCh
			}()
			req, err := http.NewRequest("GET", fmt.Sprintf("%s/cache/%d", proxy, i), nil)
			if err != nil {
				return err
			}
			req.Host = hostname
			req.Close = true
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			return res.Body.Close()
		})
	}
	if err := eg.Wait(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := rand.Intn(cacherange)
			req, err := http.NewRequest("GET", fmt.Sprintf("%s/cache/%d", proxy, i), nil)
			if err != nil {
				b.Error(err)
				return
			}
			req.Host = hostname
			req.Close = true
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				b.Error(err)
				return
			}
			res.Body.Close()
			if res.Header.Get("X-Nginx-Cache") != "HIT" {
				b.Errorf("cache miss: %v %s", req.Header, req.URL.String())
			}
		}
	})
}

func BenchmarkRC(b *testing.B) {
	hostname := "a.example.com"
	urlstr := testutil.NewUpstreamEchoNGINXServer(b, hostname)
	upstreams := map[string]string{}
	upstreams[hostname] = urlstr

	c := testutil.NewAllCache(b)
	m := rc.New(c)
	rl := testur.NewRelayer(upstreams)
	r := rp.NewRouter(rl)
	proxy := httptest.NewServer(m(r))
	b.Cleanup(func() {
		proxy.Close()
	})

	// Make cache
	const (
		concurrency = 100
		cacherange  = 10000
	)
	limitCh := make(chan struct{}, concurrency)
	eg := new(errgroup.Group)
	for i := 0; i < cacherange; i++ {
		i := i
		limitCh <- struct{}{}
		eg.Go(func() error {
			defer func() {
				<-limitCh
			}()
			req, err := http.NewRequest("GET", fmt.Sprintf("%s/cache/%d", proxy.URL, i), nil)
			if err != nil {
				return err
			}
			req.Host = hostname
			req.Close = true
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			return res.Body.Close()
		})
	}
	if err := eg.Wait(); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := rand.Intn(cacherange)
			req, err := http.NewRequest("GET", fmt.Sprintf("%s/cache/%d", proxy.URL, i), nil)
			if err != nil {
				b.Error(err)
				return
			}
			req.Host = hostname
			req.Close = true
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				b.Error(err)
				return
			}
			res.Body.Close()
			if res.Header.Get("X-Cache") != "HIT" {
				b.Errorf("cache miss: %s", req.URL.String())
			}
		}
	})
}
