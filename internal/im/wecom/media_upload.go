package wecom

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/magicyuan876/yuheng/internal/im"
	"github.com/magicyuan876/yuheng/internal/logger"
)

// WeCom intelligent-bot media upload over the WebSocket long connection.
//
// The stream reply's msg_item images are documented as unsupported in long
// connection mode, so inline pictures go a different route entirely:
//  1. aibot_upload_media_init   {type, filename, total_size, total_chunks, md5}
//     -> {upload_id}
//  2. aibot_upload_media_chunk  {upload_id, chunk_index, base64_data} × N
//     (≤512KB raw per chunk)
//  3. aibot_upload_media_finish {upload_id} -> {media_id}
//  4. aibot_respond_msg         {msgtype:"image", image:{media_id}}
//
// WeCom hosts the uploaded bytes, so image rendering is fully independent of
// the client's network reach to this deployment.

const (
	cmdUploadMediaInit   = "aibot_upload_media_init"
	cmdUploadMediaChunk  = "aibot_upload_media_chunk"
	cmdUploadMediaFinish = "aibot_upload_media_finish"

	mediaUploadChunkSize  = 512 * 1024
	mediaRequestTimeout   = 15 * time.Second
	maxMediaUploadRetries = 2
)

// pendingRequests routes server response frames back to in-flight requests
// by req_id. The read loop offers every non-callback frame here first.
type pendingRequests struct {
	mu      sync.Mutex
	waiters map[string]chan wsFrame
}

func (p *pendingRequests) add(reqID string) chan wsFrame {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.waiters == nil {
		p.waiters = make(map[string]chan wsFrame)
	}
	ch := make(chan wsFrame, 1)
	p.waiters[reqID] = ch
	return ch
}

func (p *pendingRequests) remove(reqID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.waiters, reqID)
}

// dispatch delivers a server frame to its waiter; reports whether consumed.
func (p *pendingRequests) dispatch(frame wsFrame) bool {
	reqID := frame.Headers["req_id"]
	if reqID == "" {
		return false
	}
	p.mu.Lock()
	ch, ok := p.waiters[reqID]
	if ok {
		delete(p.waiters, reqID)
	}
	p.mu.Unlock()
	if !ok {
		return false
	}
	ch <- frame
	return true
}

// request sends a frame with a fresh req_id and waits for the matching
// server response frame.
func (c *LongConnClient) request(ctx context.Context, cmd string, body any) (*wsFrame, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal %s body: %w", cmd, err)
	}
	reqID := fmt.Sprintf("%s_%d", cmd, c.reqSeq.Add(1))
	ch := c.pending.add(reqID)
	defer c.pending.remove(reqID)

	frame := wsFrame{
		Cmd:     cmd,
		Headers: map[string]string{"req_id": reqID},
		Body:    bodyBytes,
	}
	if err := c.writeJSON(frame); err != nil {
		return nil, fmt.Errorf("send %s: %w", cmd, err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(mediaRequestTimeout):
		return nil, fmt.Errorf("%s: no response within %s", cmd, mediaRequestTimeout)
	case resp := <-ch:
		if resp.ErrCode != 0 {
			return nil, fmt.Errorf("%s rejected: errcode=%d errmsg=%s", cmd, resp.ErrCode, resp.ErrMsg)
		}
		return &resp, nil
	}
}

// UploadImage uploads one image via the chunked media upload and returns its
// temporary media_id.
func (c *LongConnClient) UploadImage(ctx context.Context, img im.ReplyImage, filename string) (string, error) {
	total := len(img.Data)
	if total == 0 {
		return "", fmt.Errorf("empty image")
	}
	totalChunks := (total + mediaUploadChunkSize - 1) / mediaUploadChunkSize
	if totalChunks > 100 {
		return "", fmt.Errorf("image too large: %d chunks exceeds protocol maximum of 100", totalChunks)
	}

	initResp, err := c.request(ctx, cmdUploadMediaInit, map[string]any{
		"type":         "image",
		"filename":     filename,
		"total_size":   total,
		"total_chunks": totalChunks,
		"md5":          img.MD5,
	})
	if err != nil {
		return "", err
	}
	var initBody struct {
		UploadID string `json:"upload_id"`
	}
	if err := json.Unmarshal(initResp.Body, &initBody); err != nil || initBody.UploadID == "" {
		return "", fmt.Errorf("upload init returned no upload_id (body=%s)", truncateForLog(string(initResp.Body), 200))
	}

	for idx := 0; idx < totalChunks; idx++ {
		start := idx * mediaUploadChunkSize
		end := min(start+mediaUploadChunkSize, total)
		chunkBody := map[string]any{
			"upload_id":   initBody.UploadID,
			"chunk_index": idx,
			"base64_data": base64.StdEncoding.EncodeToString(img.Data[start:end]),
		}
		var chunkErr error
		for attempt := 0; attempt <= maxMediaUploadRetries; attempt++ {
			if _, chunkErr = c.request(ctx, cmdUploadMediaChunk, chunkBody); chunkErr == nil {
				break
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 500 * time.Millisecond):
			}
		}
		if chunkErr != nil {
			return "", fmt.Errorf("chunk %d/%d: %w", idx+1, totalChunks, chunkErr)
		}
	}

	finishResp, err := c.request(ctx, cmdUploadMediaFinish, map[string]any{
		"upload_id": initBody.UploadID,
	})
	if err != nil {
		return "", err
	}
	var finishBody struct {
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(finishResp.Body, &finishBody); err != nil || finishBody.MediaID == "" {
		return "", fmt.Errorf("upload finish returned no media_id (body=%s)", truncateForLog(string(finishResp.Body), 200))
	}
	return finishBody.MediaID, nil
}

// respondImage sends one image reply on the incoming message's req_id.
func (c *LongConnClient) respondImage(incoming *im.IncomingMessage, mediaID string) error {
	var reqID string
	if incoming.Extra != nil {
		reqID = incoming.Extra["req_id"]
	}
	if reqID == "" {
		return fmt.Errorf("missing req_id in incoming message extra")
	}
	body, err := json.Marshal(map[string]any{
		"msgtype": "image",
		"image":   map[string]string{"media_id": mediaID},
	})
	if err != nil {
		return fmt.Errorf("marshal image reply: %w", err)
	}
	return c.writeJSON(wsFrame{
		Cmd:     cmdResponse,
		Headers: map[string]string{"req_id": reqID},
		Body:    body,
	})
}

// sendReplyImages uploads and sends every inline image as its own image
// message. A per-image failure is logged and skipped — the text answer has
// already been delivered.
func (c *LongConnClient) sendReplyImages(ctx context.Context, incoming *im.IncomingMessage, images []im.ReplyImage) {
	for i, img := range images {
		mediaID, err := c.UploadImage(ctx, img, fmt.Sprintf("reply_%d.jpg", i+1))
		if err != nil {
			logger.Warnf(ctx, "[WeCom] upload reply image %d/%d failed: %v", i+1, len(images), err)
			continue
		}
		if err := c.respondImage(incoming, mediaID); err != nil {
			logger.Warnf(ctx, "[WeCom] send reply image %d/%d failed: %v", i+1, len(images), err)
			continue
		}
		logger.Infof(ctx, "[WeCom] reply image %d/%d sent: media_id=%s size=%d",
			i+1, len(images), mediaID, len(img.Data))
	}
}
