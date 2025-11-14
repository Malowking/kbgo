package chat

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Malowking/kbgo/internal/dao"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/wangle201210/chat-history/eino"
)

var chat *Chat

type Chat struct {
	cm     model.BaseChatModel
	eh     *eino.History
	params ModelParams // 添加推理参数配置
}

func GetChat() *Chat {
	return chat
}

// 暂时用不上chat功能，先不init
func init() {
	ctx := gctx.New()

	// 加载聊天配置
	var chatCfg ChatConfig
	err := g.Cfg().MustGet(ctx, "chat").Scan(&chatCfg)
	if err != nil {
		g.Log().Fatalf(ctx, "load chat config failed, err=%v", err)
		return
	}

	c, err := newChat(&openai.ChatModelConfig{
		APIKey:  chatCfg.APIKey,
		BaseURL: chatCfg.BaseURL,
		Model:   chatCfg.Model,
	}, chatCfg.ModelParams)

	if err != nil {
		g.Log().Fatalf(ctx, "newChat failed, err=%v", err)
		return
	}
	c.eh = eino.NewEinoHistory(dao.GetDsn())
	chat = c
}

func newChat(cfg *openai.ChatModelConfig, params ModelParams) (res *Chat, err error) {
	chatModel, err := openai.NewChatModel(context.Background(), cfg)
	if err != nil {
		return nil, err
	}

	// 如果没有设置参数，使用默认值
	if params.Temperature == nil && params.TopP == nil && params.MaxTokens == nil {
		params = GetDefaultParams()
	}

	return &Chat{
		cm:     chatModel,
		params: params,
	}, nil
}

func (x *Chat) GetAnswer(ctx context.Context, convID string, docs []*schema.Document, question string) (answer string, err error) {
	messages, err := x.docsMessages(ctx, convID, docs, question)
	if err != nil {
		return "", err
	}
	result, err := x.generate(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("生成答案失败: %w", err)
	}
	err = x.eh.SaveMessage(result, convID)
	if err != nil {
		g.Log().Error(ctx, "save assistant message err: %v", err)
		return
	}
	return result.Content, nil
}

// GetAnswerStream 流式生成答案
func (x *Chat) GetAnswerStream(ctx context.Context, convID string, docs []*schema.Document, question string) (answer *schema.StreamReader[*schema.Message], err error) {
	messages, err := x.docsMessages(ctx, convID, docs, question)
	if err != nil {
		return
	}
	ctx = context.Background()
	streamData, err := x.stream(ctx, messages)
	if err != nil {
		err = fmt.Errorf("生成答案失败: %w", err)
	}
	srs := streamData.Copy(2)
	go func() {
		// for save
		fullMsgs := make([]*schema.Message, 0)
		defer func() {
			srs[1].Close()
			fullMsg, err := schema.ConcatMessages(fullMsgs)
			if err != nil {
				g.Log().Error(ctx, "error concatenating messages: %v", err)
				return
			}
			err = x.eh.SaveMessage(fullMsg, convID)
			if err != nil {
				g.Log().Error(ctx, "save assistant message err: %v", err)
				return
			}
		}()
	outer:
		for {
			select {
			case <-ctx.Done():
				fmt.Println("context done", ctx.Err())
				return
			default:
				chunk, err := srs[1].Recv()
				if err != nil {
					if errors.Is(err, io.EOF) {
						break outer
					}
				}
				fullMsgs = append(fullMsgs, chunk)
			}
		}
	}()

	return srs[0], nil

}

func (x *Chat) generate(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
	opts := x.params.ToModelOptions()
	result, err := x.cm.Generate(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("llm generate failed: %v", err)
	}
	return result, nil
}

func (x *Chat) stream(ctx context.Context, messages []*schema.Message) (*schema.StreamReader[*schema.Message], error) {
	opts := x.params.ToModelOptions()
	result, err := x.cm.Stream(ctx, messages, opts...)
	if err != nil {
		return nil, fmt.Errorf("llm stream failed: %v", err)
	}
	return result, nil
}

func (x *Chat) Generate(ctx context.Context, convID string, docs []*schema.Document, question string, customParams *ModelParams) (string, error) {
	messages, err := x.docsMessages(ctx, convID, docs, question)
	if err != nil {
		return "", err
	}

	// 使用自定义参数或默认参数
	params := x.params
	if customParams != nil {
		params = *customParams
	}

	opts := params.ToModelOptions()
	result, err := x.cm.Generate(ctx, messages, opts...)
	if err != nil {
		return "", fmt.Errorf("生成答案失败: %w", err)
	}

	err = x.eh.SaveMessage(result, convID)
	if err != nil {
		g.Log().Error(ctx, "save assistant message err: %v", err)
		return "", err
	}

	return result.Content, nil
}

// UpdateParams 更新模型参数
func (x *Chat) UpdateParams(params ModelParams) {
	x.params = params
}

// GetParams 获取当前模型参数
func (x *Chat) GetParams() ModelParams {
	return x.params
}
