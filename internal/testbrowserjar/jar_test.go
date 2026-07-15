package testbrowserjar

import (
	"net/http"
	"net/url"
	"testing"
)

func TestSecureHostCookieIsolation(t *testing.T) {
	jar := New()
	launch, _ := url.Parse("http://ss-random.localhost:1234/")
	sibling, _ := url.Parse("http://evil.localhost:5678/")
	ip, _ := url.Parse("http://127.0.0.1:5678/")
	jar.SetCookies(launch, []*http.Cookie{{Name: "__Host-session", Value: "secret", Path: "/", Secure: true, HttpOnly: true}})
	if cookies := jar.Cookies(launch); len(cookies) != 1 || cookies[0].Value != "secret" {
		t.Fatalf("launch cookies = %#v", cookies)
	}
	if cookies := jar.Cookies(sibling); len(cookies) != 0 {
		t.Fatalf("sibling cookies = %#v", cookies)
	}
	jar.SetCookies(sibling, []*http.Cookie{{Name: "__Host-session", Value: "hostile", Domain: "localhost", Path: "/", Secure: true}})
	if cookies := jar.Cookies(launch); len(cookies) != 1 || cookies[0].Value != "secret" {
		t.Fatalf("domain cookie changed launch cookies = %#v", cookies)
	}
	jar.SetCookies(ip, []*http.Cookie{{Name: "__Host-session", Value: "hostile", Path: "/", Secure: true}})
	if cookies := jar.Cookies(ip); len(cookies) != 0 {
		t.Fatalf("insecure IP accepted Secure cookie = %#v", cookies)
	}
}
