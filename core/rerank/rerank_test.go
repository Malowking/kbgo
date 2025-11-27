package rerank

import (
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/os/gctx"
)

func TestRerank(t *testing.T) {
	ctx := gctx.New()

	// 测试配置文件读取
	cfg := GetConf(ctx)
	if cfg == nil {
		t.Fatal("Failed to get rerank config from config file")
	}

	// 验证配置是否正确读取
	if cfg.apiKey == "" {
		t.Fatal("rerank.apiKey is empty in config")
	}
	if cfg.Model == "" {
		t.Fatal("rerank.model is empty in config")
	}
	if cfg.url == "" {
		t.Fatal("rerank.baseURL is empty in config")
	}

	t.Logf("Using rerank config from file: model=%s, url=%s", cfg.Model, cfg.url)

	docs := []*schema.Document{
		{Content: "# 分布式训练技术原理- 数据并行 n- FSDP n- FSDP算法是由来自DeepSpeed的ZeroRedundancyOptimizer技术驱动的，但经过修改的设计和实现与PyTorch的其他组件保持一致。FSDP将模型实例分解为更小的单元，然后将每个单元内的所有参数扁平化和分片。分片参数在计算前按需通信和恢复，计算结束后立即丢弃。这种方法确保FSDP每次只需要实现一个单元的参数，这大大降低了峰值内存消耗。(数据并行+Parameter切分) n- DDP n- DistributedDataParallel (DDP)， **在每个设备上维护一个模型副本，并通过向后传递的集体AllReduce操作同步梯度，从而确保在训练期间跨副本的模型一致性** 。为了加快训练速度， **DDP将梯度通信与向后计算重叠** ，促进在不同资源上并发执行工作负载。 n- ZeRO n- Model state n- Optimizer->ZeRO1 n- 将optimizer state分成若干份，每块GPU上各自维护一份"},
		{Content: "- ZeRO-Offload 分为 Offload Strategy 和 Offload Schedule 两部分，前者解决如何在 GPU 和 CPU 间划分模型的问题，后者解决如何调度计算和通信的问题 n- ZeRO-Infinity n- 一是将offload和 ZeRO 的结合从 ZeRO-2 延伸到了 ZeRO-3，解决了模型参数受限于单张 GPU 内存的问题 n- 二是解决了 ZeRO-Offload 在训练 batch size 较小的时候效率较低的问题 n- 三是除 CPU 内存外，进一步尝试利用 NVMe 的空间 n- 模型并行 n- tensor-wise parallelism n- MLP切分 n- 对第一个线性层按列切分，对第二个线性层按行切分 n-  ![图片](./img/分布式训练技术原理-幕布图片-36114-765327.jpg) n-  ![图片](./img/分布式训练技术原理-幕布图片-392521-261326.jpg) n-  ![图片](./img/分布式训练技术原理-幕布图片-57107-679259.jpg) n- self-attention切分"},
	}

	// 先测试不调用真实API的情况
	t.Log("Testing rerank configuration...")

	// 如果配置正确，尝试调用rerank API
	output, err := NewRerank(ctx, "SDP 如何通过参数分片减少 GPU 显存占用", docs, 2)
	if err != nil {
		t.Logf("Rerank API call failed (this might be expected if API is not available): %v", err)
		// 不要Fatal，因为API可能不可用，但配置读取应该成功
		return
	}

	t.Logf("Rerank successful, returned %d documents", len(output))
	for i, doc := range output {
		t.Logf("Document %d: score=%f, content_length=%d", i+1, doc.Score(), len(doc.Content))
	}
}
