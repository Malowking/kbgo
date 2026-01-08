package claude_skills

import (
	"context"
	"testing"
)

// TestSkillExecution 测试 Skill 执行
func TestSkillExecution(t *testing.T) {
	ctx := context.Background()

	// 1. 创建 Skill 管理器
	manager, err := NewSkillManager("/tmp/kbgo_venvs", "/tmp/kbgo_skills")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 2. 定义一个简单的 Skill
	skill := &Skill{
		ID:          "test_pandas",
		Name:        "Pandas 数据分析",
		Description: "使用 pandas 分析 CSV 数据",
		Version:     "1.0.0",
		Runtime: SkillRuntime{
			Type:    "python",
			Version: "3.9+",
			Requirements: []string{
				"pandas==2.0.0",
				"numpy==1.24.0",
			},
		},
		Tool: SkillTool{
			Name:        "analyze_data",
			Description: "分析数据并返回统计信息",
			Parameters: map[string]SkillToolParameter{
				"data": {
					Type:        "array",
					Required:    true,
					Description: "要分析的数据数组",
				},
			},
		},
		Script: `
import json
import sys
import pandas as pd
import numpy as np

def main(data):
    # 将数据转换为 DataFrame
    df = pd.DataFrame(data)

    # 计算统计信息
    result = {
        "shape": df.shape,
        "columns": df.columns.tolist(),
        "dtypes": df.dtypes.astype(str).to_dict(),
        "summary": df.describe().to_dict() if len(df) > 0 else {},
        "null_counts": df.isnull().sum().to_dict(),
    }

    print(json.dumps(result, ensure_ascii=False))

if __name__ == "__main__":
    args = json.loads(sys.argv[1])
    main(**args)
`,
	}

	// 3. 注册 Skill
	if err := manager.RegisterSkill(skill); err != nil {
		t.Fatalf("注册 Skill 失败: %v", err)
	}

	// 4. 准备测试数据
	testData := []map[string]interface{}{
		{"name": "Alice", "age": 25, "score": 85.5},
		{"name": "Bob", "age": 30, "score": 92.0},
		{"name": "Charlie", "age": 35, "score": 78.5},
	}

	// 5. 执行 Skill
	result, err := manager.Executor.ExecuteSkill(ctx, skill, map[string]interface{}{
		"data": testData,
	}, nil)

	if err != nil {
		t.Fatalf("执行 Skill 失败: %v", err)
	}

	// 6. 检查结果
	if !result.Success {
		t.Fatalf("Skill 执行失败: %s", result.Error)
	}

	t.Logf("执行成功 (耗时: %dms)", result.Duration)
	t.Logf("输出: %s", result.Output)
}

// TestMultipleSkills 测试多个 Skills
func TestMultipleSkills(t *testing.T) {
	ctx := context.Background()

	manager, err := NewSkillManager("/tmp/kbgo_venvs", "/tmp/kbgo_skills")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// Skill 1: 简单计算（无依赖）
	skill1 := &Skill{
		ID:          "simple_calc",
		Name:        "简单计算器",
		Description: "执行简单的数学计算",
		Version:     "1.0.0",
		Runtime: SkillRuntime{
			Type:         "python",
			Version:      "3.9+",
			Requirements: []string{}, // 无额外依赖
		},
		Tool: SkillTool{
			Name:        "calculate",
			Description: "计算表达式的值",
			Parameters: map[string]SkillToolParameter{
				"expression": {
					Type:        "string",
					Required:    true,
					Description: "数学表达式，如 '2 + 3 * 4'",
				},
			},
		},
		Script: `
import json
import sys

def main(expression):
    try:
        result = eval(expression)
        print(json.dumps({"result": result}))
    except Exception as e:
        print(json.dumps({"error": str(e)}))

if __name__ == "__main__":
    args = json.loads(sys.argv[1])
    main(**args)
`,
	}

	// Skill 2: 数据可视化（有依赖）
	skill2 := &Skill{
		ID:          "data_viz",
		Name:        "数据可视化",
		Description: "生成数据图表",
		Version:     "1.0.0",
		Runtime: SkillRuntime{
			Type:    "python",
			Version: "3.9+",
			Requirements: []string{
				"matplotlib==3.7.0",
				"numpy==1.24.0",
			},
		},
		Tool: SkillTool{
			Name:        "plot_data",
			Description: "绘制数据图表",
			Parameters: map[string]SkillToolParameter{
				"data": {
					Type:        "array",
					Required:    true,
					Description: "要绘制的数据",
				},
				"output_path": {
					Type:        "string",
					Required:    true,
					Description: "输出图片路径",
				},
			},
		},
		Script: `
import json
import sys
import matplotlib.pyplot as plt
import numpy as np

def main(data, output_path):
    plt.figure(figsize=(10, 6))
    plt.plot(data)
    plt.title("Data Visualization")
    plt.xlabel("Index")
    plt.ylabel("Value")
    plt.grid(True)
    plt.savefig(output_path)

    print(json.dumps({"success": True, "output": output_path}))

if __name__ == "__main__":
    args = json.loads(sys.argv[1])
    main(**args)
`,
	}

	// 注册两个 Skills
	manager.RegisterSkill(skill1)
	manager.RegisterSkill(skill2)

	// 测试 Skill 1（无依赖，应该很快）
	t.Run("SimpleCalc", func(t *testing.T) {
		result, err := manager.Executor.ExecuteSkill(ctx, skill1, map[string]interface{}{
			"expression": "2 + 3 * 4",
		}, nil)
		if err != nil {
			t.Fatalf("执行失败: %v", err)
		}
		t.Logf("计算结果: %s (耗时: %dms)", result.Output, result.Duration)
	})

	// 测试 Skill 2（有依赖，首次会慢一些）
	t.Run("DataViz", func(t *testing.T) {
		result, err := manager.Executor.ExecuteSkill(ctx, skill2, map[string]interface{}{
			"data":        []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			"output_path": "/tmp/test_plot.png",
		}, nil)
		if err != nil {
			t.Fatalf("执行失败: %v", err)
		}
		t.Logf("可视化结果: %s (耗时: %dms)", result.Output, result.Duration)
	})

	// 获取所有 LLM 工具定义
	tools := manager.GetLLMTools()
	t.Logf("注册了 %d 个工具", len(tools))
	for _, tool := range tools {
		t.Logf("- %s: %s", tool.Name, tool.Desc)
	}
}
