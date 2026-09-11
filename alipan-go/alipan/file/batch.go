package file

import (
	"context"
	"encoding/json"

	"github.com/langhuachuanshi/alipan-go/alipan/invoker"
	"github.com/langhuachuanshi/alipan-go/alipan/types"
)

// BatchSubRequest 批量子请求。
type BatchSubRequest struct {
	Body    map[string]any      `json:"body"`
	ID      string              `json:"id"`
	URL     string              `json:"url"`
	Headers map[string]string   `json:"headers"`
	Method  string              `json:"method"`
}

// BatchItem 批量响应里的单项（body 延迟解码）。
type BatchItem struct {
	ID     string          `json:"id"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
	Method string          `json:"method"`
}

type batchRequestPayload struct {
	Requests []BatchSubRequest `json:"requests"`
	Resource string            `json:"resource"`
}

// batch 执行批量请求，按 100 一组切片。返回每个子响应的原始项。
//
// resource 默认 "file"。useV2=true 时走 /adrive/v2/batch（保存分享用），否则 /v3/batch。
func (s *Service) batch(ctx context.Context, subURL, resource string, ids []string, bodies []map[string]any, useV2 bool) ([]BatchItem, error) {
	if len(ids) != len(bodies) {
		return nil, invoker.NewAPIError(0, "InvalidArgument", "ids and bodies length mismatch")
	}
	if resource == "" {
		resource = "file"
	}
	subs := make([]BatchSubRequest, 0, len(ids))
	for i := range ids {
		subs = append(subs, BatchSubRequest{
			Body:    bodies[i],
			ID:      ids[i],
			URL:     subURL,
			Method:  "POST",
			Headers: map[string]string{"Content-Type": "application/json"},
		})
	}
	path := pathBatch
	if useV2 {
		path = pathBatchV2
	}
	var allResults []BatchItem
	for start := 0; start < len(subs); start += batchMaxPerRequest {
		end := start + batchMaxPerRequest
		if end > len(subs) {
			end = len(subs)
		}
		payload := batchRequestPayload{Requests: subs[start:end], Resource: resource}
		var resp struct {
			Responses []BatchItem `json:"responses"`
		}
		if err := invoker.PostAndDecode(ctx, s.inv, path, payload, &resp, []int{200}); err != nil {
			return nil, err
		}
		allResults = append(allResults, resp.Responses...)
	}
	return allResults, nil
}

// DecodeBatchItem 把子响应 body 解码到目标类型。
func DecodeBatchItem[T any](item BatchItem) (string, int, *T, error) {
	var t T
	if len(item.Body) > 0 {
		if err := json.Unmarshal(item.Body, &t); err != nil {
			return item.ID, item.Status, nil, err
		}
	}
	return item.ID, item.Status, &t, nil
}

// _ 用于确保 types 包被引用（部分方法用到 types.BaseFile）。
var _ = types.FileTypeFile
