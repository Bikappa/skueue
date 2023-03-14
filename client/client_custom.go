package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/beyondan/gqlgenc/client"
	"github.com/beyondan/gqlgenc/client/transport"
)

type ExtendedClient struct {
	host   string
	userId string
	*Client
}

func (c *ExtendedClient) GetCompilationTarball(ctx context.Context, compilationId string) (io.Reader, error) {
	httpClient := http.Client{}

	query := url.Values{
		"compilationId": []string{compilationId},
	}
	url := url.URL{
		Scheme:   "http",
		Host:     c.host,
		Path:     "artefacts",
		RawQuery: query.Encode(),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), bytes.NewBufferString(""))
	if err != nil {
		return nil, err
	}
	req.Header.Set("arduino-user-id", c.userId)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func NewClient(host, userId string) (*ExtendedClient, func(ctx context.Context)) {
	wstr := &transport.Ws{
		URL: fmt.Sprintf("ws://%s/query", host),
		ConnectionParams: map[string]string{
			"arduino-user-id": userId,
		},
		WebsocketConnProvider: transport.DefaultWebsocketConnProvider(time.Second * 20),
	}

	httptr := &transport.Http{
		URL: fmt.Sprintf("http://%s/query", host),
		RequestOptions: []transport.HttpRequestOption{
			func(req *http.Request) {
				req.Header.Set("arduino-user-id", userId)
			},
		},
	}

	tr := transport.SplitSubscription(wstr, httptr)

	gql := &Client{
		Client: &client.Client{
			Transport: tr,
		},
	}
	client := &ExtendedClient{
		userId: userId,
		host:   host,
		Client: gql,
	}
	return client, func(ctx context.Context) {
		wstr.Start(ctx)
	}
}
