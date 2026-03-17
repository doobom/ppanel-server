package resend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const apiURL = "https://api.resend.com/emails"

type Client struct {
	conf Config
}

type Config struct {
	APIKey   string `json:"api_key"`
	From     string `json:"from"`
	SiteName string `json:"siteName"`
}

type sendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

type sendResponse struct {
	ID    string `json:"id"`
	Error string `json:"message,omitempty"`
}

func NewClient(conf *Config) *Client {
	return &Client{conf: *conf}
}

func (c *Client) Send(to []string, subject, body string) error {
	from := c.conf.From
	if c.conf.SiteName != "" {
		from = fmt.Sprintf("%s <%s>", c.conf.SiteName, c.conf.From)
	}

	payload := sendRequest{
		From:    from,
		To:      to,
		Subject: subject,
		HTML:    body,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request failed: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.conf.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send email err: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var errResp sendResponse
		_ = json.Unmarshal(respBody, &errResp)
		return fmt.Errorf("resend api error %d: %s", resp.StatusCode, errResp.Error)
	}

	return nil
}