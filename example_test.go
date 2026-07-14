package singleserve_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rztaylor/singleserve"
)

func Example() {
	server, err := singleserve.New(singleserve.Options{
		Handler:  http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, "hello") }),
		Lifetime: singleserve.ExplicitLifetime(),
		Opener:   singleserve.BrowserOpenerFunc(func(context.Context, string) error { return nil }),
	})
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = server.Start(ctx)
	fmt.Println(err)
	// Output: context canceled
}
