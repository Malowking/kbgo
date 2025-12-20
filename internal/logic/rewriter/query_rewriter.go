package rewriter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Malowking/kbgo/core/formatter"
	coreModel "github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
)

// QueryRewriter 查询重写器（用于指代消解）
type QueryRewriter struct {
	cache           *gcache.Cache
	cacheExpire     time.Duration
	maxContextTurns int
}

// NewQueryRewriter 创建查询重写器
func NewQueryRewriter() *QueryRewriter {
	return &QueryRewriter{
		cache:           gcache.New(),
		cacheExpire:     time.Minute * 5, // 缓存5分钟
		maxContextTurns: 3,               // 默认使用最近3轮对话
	}
}

// Config 配置选项
type Config struct {
	Enable          bool    // 是否启用查询重写
	ModelID         string  // 用于重写的模型ID
	MaxContextTurns int     // 最多使用几轮历史对话
	Temperature     float32 // 模型温度
	UseCache        bool    // 是否使用缓存
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enable:          true,
		ModelID:         "", // 使用与聊天相同的模型
		MaxContextTurns: 3,
		Temperature:     0.3,
		UseCache:        true,
	}
}

// RewriteQuery 重写查询（指代消解）
func (r *QueryRewriter) RewriteQuery(ctx context.Context, currentQuery string, chatHistory []*schema.Message, config *Config) (string, error) {
	// 如果未启用，直接返回原查询
	if !config.Enable {
		return currentQuery, nil
	}

	// 检查是否配置了重写模型
	if !coreModel.Registry.HasRewriteModel() {
		g.Log().Debugf(ctx, "未配置重写模型，跳过查询重写")
		return currentQuery, nil
	}

	// 如果没有历史对话，不需要重写
	if len(chatHistory) == 0 {
		g.Log().Debugf(ctx, "没有历史对话，跳过查询重写")
		return currentQuery, nil
	}

	// 检查是否需要重写
	if !r.needsRewriting(currentQuery) {
		g.Log().Debugf(ctx, "查询不需要重写，跳过")
		return currentQuery, nil
	}

	// 尝试从缓存获取
	if config.UseCache {
		cacheKey := r.buildCacheKey(currentQuery, chatHistory)
		if cached, err := r.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			rewritten := cached.String()
			g.Log().Infof(ctx, "查询重写（缓存命中）: [%s] -> [%s]", currentQuery, rewritten)
			return rewritten, nil
		}
	}

	// 调用 LLM 进行重写
	rewritten, err := r.rewriteWithLLM(ctx, currentQuery, chatHistory, config)
	if err != nil {
		g.Log().Warningf(ctx, "查询重写失败: %v，使用原查询", err)
		return currentQuery, nil // 失败时返回原查询，不中断流程
	}

	// 缓存结果
	if config.UseCache {
		cacheKey := r.buildCacheKey(currentQuery, chatHistory)
		r.cache.Set(ctx, cacheKey, rewritten, r.cacheExpire)
	}

	g.Log().Infof(ctx, "查询重写成功: [%s] -> [%s]", currentQuery, rewritten)
	return rewritten, nil
}

// needsRewriting 判断是否需要重写
func (r *QueryRewriter) needsRewriting(query string) bool {
	// 检测代词
	pronouns := []string{
		"它", "他", "她", "这个", "那个", "这些", "那些",
		"它的", "他的", "她的", "这个的", "那个的",
		"它们", "他们", "她们",
	}

	for _, pronoun := range pronouns {
		if strings.Contains(query, pronoun) {
			return true
		}
	}

	// 检测省略或不完整的问句开头
	incompletePatterns := []string{
		"那", "还有", "除此之外", "另外", "此外",
		"那么", "还", "再", "继续",
		"呢", "吗", "么",
	}

	queryTrimmed := strings.TrimSpace(query)
	for _, pattern := range incompletePatterns {
		if strings.HasPrefix(queryTrimmed, pattern) {
			return true
		}
	}

	// 检测疑问词开头但可能省略主语的情况
	questionStarters := []string{
		"有什么", "有哪些", "怎么", "如何", "为什么",
		"什么时候", "在哪", "哪里", "多少", "几个",
	}

	for _, starter := range questionStarters {
		if strings.HasPrefix(queryTrimmed, starter) {
			// 如果问题很短（< 15字），可能省略了主语
			if len([]rune(query)) < 15 {
				return true
			}
		}
	}

	return false
}

// rewriteWithLLM 使用 LLM 重写查询
func (r *QueryRewriter) rewriteWithLLM(ctx context.Context, currentQuery string, chatHistory []*schema.Message, config *Config) (string, error) {
	// 构建系统提示词
	systemPrompt := `你是一个对话理解助手。你的任务是将用户的问题改写为完整的、独立的、易于理解的问题。

核心规则：
1. 如果问题中有代词（它、他、她、这个、那个等），根据对话历史替换为具体的名词
2. 如果问题省略了主语或宾语，根据上下文补充完整
3. 保持问题的原意和语气，不要改变用户的意图
4. 不要添加用户没有问到的内容
5. 如果问题已经完整且清晰，保持原样
6. 改写后的问题应该是一个完整的句子

示例1：
对话历史:
用户: 什么是机器学习?
AI: 机器学习是人工智能的一个分支，它让计算机能够从数据中学习...
当前问题: "它的应用有哪些?"
改写: "机器学习的应用有哪些?"

示例2：
对话历史:
用户: Python和Java哪个更适合初学者?
AI: 对于初学者来说，Python通常更容易上手...
当前问题: "那学Java需要什么基础?"
改写: "学习Java需要什么基础?"

示例3：
对话历史:
用户: 介绍一下GPT-4
AI: GPT-4是OpenAI开发的大型语言模型...
当前问题: "它和GPT-3有什么区别?"
改写: "GPT-4和GPT-3有什么区别?"

示例4：
对话历史:
用户: 如何学习深度学习?
AI: 学习深度学习需要掌握数学基础、Python编程...
当前问题: "需要多长时间?"
改写: "学习深度学习需要多长时间?"

请直接输出改写后的问题，不要有任何额外的解释、标点符号或格式。`

	// 构建对话历史上下文
	contextBuilder := strings.Builder{}
	contextBuilder.WriteString("对话历史:\n")

	// 只取最近的 N 轮对话
	maxTurns := config.MaxContextTurns
	if maxTurns <= 0 {
		maxTurns = r.maxContextTurns
	}

	startIdx := 0
	if len(chatHistory) > maxTurns*2 {
		startIdx = len(chatHistory) - maxTurns*2
	}

	recentHistory := chatHistory[startIdx:]
	for _, msg := range recentHistory {
		role := "用户"
		if msg.Role == schema.Assistant {
			role = "AI"
		} else if msg.Role == schema.System {
			continue // 跳过系统消息
		}

		// 截断过长的消息（保留前200字）
		content := msg.Content
		if len([]rune(content)) > 200 {
			content = string([]rune(content)[:200]) + "..."
		}

		contextBuilder.WriteString(fmt.Sprintf("%s: %s\n", role, content))
	}

	// 构建用户提示
	userPrompt := fmt.Sprintf("%s\n当前问题: \"%s\"\n改写:", contextBuilder.String(), currentQuery)

	// 构建消息列表
	messages := []*schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	}

	// 获取重写模型配置（从内存中获取）
	mc := coreModel.Registry.GetRewriteModel()
	if mc == nil {
		return "", fmt.Errorf("重写模型未配置")
	}

	g.Log().Debugf(ctx, "使用重写模型: %s (%s)", mc.Name, mc.ModelID)

	// 选择格式适配器
	var msgFormatter formatter.MessageFormatter
	if strings.HasPrefix(strings.ToLower(mc.Name), "qwen") {
		msgFormatter = formatter.NewQwenFormatter()
	} else {
		msgFormatter = formatter.NewOpenAIFormatter()
	}

	// 创建模型服务
	modelService := coreModel.NewModelService(mc.APIKey, mc.BaseURL, msgFormatter)

	// 调用模型
	startTime := time.Now()
	resp, err := modelService.ChatCompletion(ctx, coreModel.ChatCompletionParams{
		ModelName:           mc.Name,
		Messages:            messages,
		Temperature:         config.Temperature,
		MaxCompletionTokens: 200, // 改写后的问题通常不长
	})

	if err != nil {
		return "", fmt.Errorf("调用模型失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("模型返回空响应")
	}

	// 提取重写后的问题
	rewrittenQuery := strings.TrimSpace(resp.Choices[0].Message.Content)

	// 清理可能的多余标点符号
	rewrittenQuery = strings.Trim(rewrittenQuery, `"'「」『』【】""''`)

	// 如果重写失败或返回空，使用原查询
	if rewrittenQuery == "" || len([]rune(rewrittenQuery)) < 2 {
		return currentQuery, fmt.Errorf("重写结果为空")
	}

	// 记录性能指标
	duration := time.Since(startTime)
	g.Log().Debugf(ctx, "查询重写耗时: %v, tokens: %d", duration, resp.Usage.TotalTokens)

	return rewrittenQuery, nil
}

// buildCacheKey 构建缓存键
func (r *QueryRewriter) buildCacheKey(query string, history []*schema.Message) string {
	// 使用最后几条消息的内容哈希作为缓存键
	var keyBuilder strings.Builder
	keyBuilder.WriteString(query)

	// 只使用最后2轮对话作为缓存键的一部分
	start := 0
	if len(history) > 4 {
		start = len(history) - 4
	}

	for i := start; i < len(history); i++ {
		keyBuilder.WriteString("|")
		keyBuilder.WriteString(string(history[i].Role))
		keyBuilder.WriteString(":")
		// 只使用前50个字符
		content := history[i].Content
		if len(content) > 50 {
			content = content[:50]
		}
		keyBuilder.WriteString(content)
	}

	return fmt.Sprintf("query_rewrite:%s", keyBuilder.String())
}

// RewriteResult 重写结果
type RewriteResult struct {
	Original    string        // 原始查询
	Rewritten   string        // 重写后的查询
	IsRewritten bool          // 是否被重写
	Duration    time.Duration // 重写耗时
}

// RewriteQueryWithResult 重写查询并返回详细结果
func (r *QueryRewriter) RewriteQueryWithResult(ctx context.Context, currentQuery string, chatHistory []*schema.Message, config *Config) (*RewriteResult, error) {
	startTime := time.Now()

	result := &RewriteResult{
		Original:    currentQuery,
		Rewritten:   currentQuery,
		IsRewritten: false,
	}

	if !config.Enable || len(chatHistory) == 0 {
		result.Duration = time.Since(startTime)
		return result, nil
	}

	rewritten, err := r.RewriteQuery(ctx, currentQuery, chatHistory, config)

	result.Duration = time.Since(startTime)
	result.Rewritten = rewritten
	result.IsRewritten = (rewritten != currentQuery)

	return result, err
}
