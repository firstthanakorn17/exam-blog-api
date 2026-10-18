package webhook

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Notifier struct {
	webhookURL string
	secret     string
	client     *http.Client
}

func NewNotifier(webhookURL, secret string) *Notifier {
	return &Notifier{
		webhookURL: webhookURL,
		secret:     secret,
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// NotifyTags fires an asynchronous webhook to exam-blog-interface (/api/revalidate)
// to invalidate Next.js 16 cacheTag entries immediately when data changes in Go API.
func (n *Notifier) NotifyTags(tags ...string) {
	if n.webhookURL == "" || len(tags) == 0 {
		return
	}

	go func() {
		for _, tag := range tags {
			payload, _ := json.Marshal(map[string]string{
				"tag":    tag,
				"secret": n.secret,
			})

			req, err := http.NewRequest(http.MethodPost, n.webhookURL, bytes.NewBuffer(payload))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-revalidate-secret", n.secret)

			resp, err := n.client.Do(req)
			if err != nil {
				log.Printf("[webhook] reader revalidate skipped (%s): %v", tag, err)
				continue
			}
			_ = resp.Body.Close()
		}
	}()
}
