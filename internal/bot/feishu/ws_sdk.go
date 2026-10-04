//go:build !arm && !386 && !mips && !mipsle

package feishu

import (
	"context"

	"reasonix/internal/bot"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

func (a *adapter) runWebSocket(ctx context.Context) {
	secret, err := a.appSecret()
	if err != nil {
		a.logger.Error("feishu websocket config error", "err", err)
		return
	}
	eventHandler := a.newEventDispatcher()
	bot.RunWithRetry(ctx, a.logger, "feishu sdk websocket", bot.RetryConfig{}, func(ctx context.Context) error {
		opts := []larkws.ClientOption{
			larkws.WithEventHandler(eventHandler),
			larkws.WithLogLevel(larkcore.LogLevelError),
			larkws.WithAutoReconnect(true),
			larkws.WithOnReady(func() { a.logger.Info("feishu sdk websocket connected") }),
			larkws.WithOnReconnecting(func() { a.logger.Warn("feishu sdk websocket reconnecting") }),
			larkws.WithOnReconnected(func() { a.logger.Info("feishu sdk websocket reconnected") }),
			larkws.WithOnError(func(err error) { a.logger.Error("feishu sdk websocket error", "err", err) }),
		}
		if feishuDomain(a.cfg.Domain) == "lark" {
			opts = append(opts, larkws.WithDomain(larkOpenBaseURL))
		}
		client := larkws.NewClient(a.cfg.AppID, secret, opts...)
		a.wsClient = client
		errCh := make(chan error, 1)
		go func() { errCh <- client.Start(ctx) }()
		select {
		case <-ctx.Done():
			client.Close()
			return nil
		case err := <-errCh:
			client.Close()
			return err
		}
	})
}

func (a *adapter) newEventDispatcher() *dispatcher.EventDispatcher {
	return dispatcher.NewEventDispatcher(a.cfg.VerificationToken, "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			a.handleSDKMessage(ctx, event)
			return nil
		}).
		OnP2MessageReadV1(func(ctx context.Context, event *larkim.P2MessageReadV1) error {
			return nil
		}).
		OnP2MessageReactionCreatedV1(func(ctx context.Context, event *larkim.P2MessageReactionCreatedV1) error {
			return nil
		}).
		OnP2MessageReactionDeletedV1(func(ctx context.Context, event *larkim.P2MessageReactionDeletedV1) error {
			return nil
		}).
		OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			return a.handleSDKCardAction(event)
		})
}
