//go:build arm || 386 || mips || mipsle

package feishu

import (
	"context"
	"encoding/json"
	"fmt"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type feishuEventDispatcher struct {
	adapter *adapter
}

func (a *adapter) runWebSocket(ctx context.Context) {
	if _, err := a.appSecret(); err != nil {
		a.logger.Error("feishu websocket config error", "err", err)
		return
	}
	a.logger.Error("feishu sdk websocket is unavailable on 32-bit architectures; configure webhook mode")
	<-ctx.Done()
}

func (a *adapter) newEventDispatcher() *feishuEventDispatcher {
	return &feishuEventDispatcher{adapter: a}
}

func (d *feishuEventDispatcher) Do(ctx context.Context, payload []byte) (any, error) {
	fuzzy := &larkevent.EventFuzzy{}
	if err := json.Unmarshal(payload, fuzzy); err != nil {
		return nil, fmt.Errorf("event json unmarshal, err: %w", err)
	}
	var eventType string
	if fuzzy.Event != nil {
		if et, ok := fuzzy.Event.Type.(string); ok {
			eventType = et
		}
	}
	if fuzzy.Header != nil {
		eventType = fuzzy.Header.EventType
	}
	req := &larkevent.EventReq{Body: payload}
	switch eventType {
	case "card.action.trigger":
		ev := &callback.CardActionTriggerEvent{}
		if err := json.Unmarshal(payload, ev); err != nil {
			return nil, err
		}
		ev.EventReq = req
		return d.adapter.handleSDKCardAction(ev)
	case "im.message.receive_v1":
		ev := &larkim.P2MessageReceiveV1{}
		if err := json.Unmarshal(payload, ev); err != nil {
			return nil, err
		}
		ev.EventReq = req
		d.adapter.handleSDKMessage(ctx, ev)
		return nil, nil
	case "im.message.message_read_v1", "im.message.reaction.created_v1", "im.message.reaction.deleted_v1":
		return nil, nil
	default:
		return nil, fmt.Errorf("event type: %s, not found handler", eventType)
	}
}
