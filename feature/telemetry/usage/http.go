package usage

import (
	"bytes"
	"fmt"
	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/internal/json"
	"net/http"
)

type Client struct {
	HTTPClient
	SessionEndpoint string
	ReportEndpoint  string
}

func (c *Client) NewSession(ctx context.Context, in *SessionRequest) (*SessionReply, error) {
	reply := &SessionReply{}
	err := c.Send(ctx, c.SessionEndpoint, in, reply)
	return reply, err
}

func (c *Client) SendReport(ctx context.Context, in *ReportRequest) (*ReportReply, error) {
	reply := &ReportReply{}
	err := c.Send(ctx, c.ReportEndpoint, in, reply)
	return reply, err
}

type HTTPClient interface {
	Send(context.Context, string, any, any) error
}

type BasicClient struct {
	Client    *http.Client
	URL       string
	UserAgent string
}

func (c *BasicClient) Send(ctx context.Context, path string, in, out any) error {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(in); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.URL+path, buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := c.Client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
