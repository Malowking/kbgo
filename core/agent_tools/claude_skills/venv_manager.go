package claude_skills

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// VenvManager 虚拟环境管理器
type VenvManager struct {
	baseDir    string               // 虚拟环境根目录
	venvCache  map[string]*VenvInfo // 虚拟环境缓存
	cacheMutex sync.RWMutex         // 缓存锁
}

// VenvInfo 虚拟环境信息
type VenvInfo struct {
	Hash         string    // 依赖哈希
	Path         string    // 虚拟环境路径
	PythonPath   string    // Python 解释器路径
	Requirements []string  // 依赖列表
	CreatedAt    time.Time // 创建时间
	LastUsedAt   time.Time // 最后使用时间
}

// NewVenvManager 创建虚拟环境管理器
func NewVenvManager(baseDir string) (*VenvManager, error) {
	// 确保基础目录存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("创建虚拟环境目录失败: %w", err)
	}

	return &VenvManager{
		baseDir:   baseDir,
		venvCache: make(map[string]*VenvInfo),
	}, nil
}

// GetOrCreateVenv 获取或创建虚拟环境（带进度回调）
func (m *VenvManager) GetOrCreateVenv(ctx context.Context, requirements []string, progressCallback ProgressCallback) (*VenvInfo, error) {
	// 1. 计算依赖哈希
	hash := m.computeHash(requirements)

	// 2. 检查缓存
	m.cacheMutex.RLock()
	if venv, exists := m.venvCache[hash]; exists {
		m.cacheMutex.RUnlock()
		// 更新最后使用时间
		venv.LastUsedAt = time.Now()
		g.Log().Infof(ctx, "[虚拟环境] 使用缓存: %s", hash[:8])

		// 发送使用缓存进度
		if progressCallback != nil {
			progressCallback("venv_cached", "使用已缓存的虚拟环境", map[string]interface{}{
				"venv_hash": hash[:8],
			})
		}

		return venv, nil
	}
	m.cacheMutex.RUnlock()

	// 3. 创建新的虚拟环境
	g.Log().Infof(ctx, "[虚拟环境] 创建新环境: %s", hash[:8])
	venv, err := m.createVenv(ctx, hash, requirements, progressCallback)
	if err != nil {
		return nil, err
	}

	// 4. 加入缓存
	m.cacheMutex.Lock()
	m.venvCache[hash] = venv
	m.cacheMutex.Unlock()

	return venv, nil
}

// createVenv 创建虚拟环境（带进度回调）
func (m *VenvManager) createVenv(ctx context.Context, hash string, requirements []string, progressCallback ProgressCallback) (*VenvInfo, error) {
	venvPath := filepath.Join(m.baseDir, hash)

	// 1. 检查是否已存在（可能是之前创建的）
	pythonPath := filepath.Join(venvPath, "bin", "python")
	if _, err := os.Stat(pythonPath); err == nil {
		g.Log().Infof(ctx, "[虚拟环境] 发现已存在的环境: %s", hash[:8])

		// 发送发现已存在环境进度
		if progressCallback != nil {
			progressCallback("venv_exists", "发现已存在的虚拟环境", map[string]interface{}{
				"venv_hash": hash[:8],
			})
		}

		return &VenvInfo{
			Hash:         hash,
			Path:         venvPath,
			PythonPath:   pythonPath,
			Requirements: requirements,
			CreatedAt:    time.Now(),
			LastUsedAt:   time.Now(),
		}, nil
	}

	// 2. 创建虚拟环境
	g.Log().Infof(ctx, "[虚拟环境] 执行: python3 -m venv %s", venvPath)

	// 发送创建虚拟环境进度
	if progressCallback != nil {
		progressCallback("creating_venv", "正在创建虚拟环境...", map[string]interface{}{
			"venv_hash": hash[:8],
			"venv_path": venvPath,
		})
	}

	cmd := exec.CommandContext(ctx, "python3", "-m", "venv", venvPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("创建虚拟环境失败: %w, output: %s", err, string(output))
	}

	g.Log().Infof(ctx, "[虚拟环境] 创建成功: %s", hash[:8])

	// 3. 安装依赖
	if len(requirements) > 0 {
		g.Log().Infof(ctx, "[虚拟环境] 安装 %d 个依赖包", len(requirements))

		// 发送开始安装依赖进度
		if progressCallback != nil {
			progressCallback("installing_deps", fmt.Sprintf("正在安装 %d 个依赖包...", len(requirements)), map[string]interface{}{
				"requirements_count": len(requirements),
				"requirements":       requirements,
			})
		}

		pipPath := filepath.Join(venvPath, "bin", "pip")

		// 升级 pip
		if progressCallback != nil {
			progressCallback("upgrading_pip", "正在升级 pip...", nil)
		}

		cmd = exec.CommandContext(ctx, pipPath, "install", "--upgrade", "pip")
		if output, err := cmd.CombinedOutput(); err != nil {
			g.Log().Warningf(ctx, "升级 pip 失败: %v, output: %s", err, string(output))
		}

		// 安装依赖（批量安装）
		args := []string{"install"}
		args = append(args, requirements...)

		cmd = exec.CommandContext(ctx, pipPath, args...)
		cmd.Stdout = os.Stdout // 实时输出安装进度
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("安装依赖失败: %w", err)
		}

		// 发送依赖安装完成进度
		if progressCallback != nil {
			progressCallback("deps_installed", "依赖包安装完成", map[string]interface{}{
				"requirements_count": len(requirements),
			})
		}
	}

	venv := &VenvInfo{
		Hash:         hash,
		Path:         venvPath,
		PythonPath:   pythonPath,
		Requirements: requirements,
		CreatedAt:    time.Now(),
		LastUsedAt:   time.Now(),
	}

	g.Log().Infof(ctx, "[虚拟环境] 创建成功: %s", hash[:8])

	// 发送虚拟环境创建完成进度
	if progressCallback != nil {
		progressCallback("venv_ready", "虚拟环境准备完成", map[string]interface{}{
			"venv_hash": hash[:8],
		})
	}

	return venv, nil
}

// computeHash 计算依赖哈希
func (m *VenvManager) computeHash(requirements []string) string {
	// 排序后计算哈希，确保相同依赖生成相同哈希
	sorted := make([]string, len(requirements))
	copy(sorted, requirements)

	// 简单排序
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	data := strings.Join(sorted, "\n")
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// CleanupOldVenvs 清理旧的虚拟环境
func (m *VenvManager) CleanupOldVenvs(ctx context.Context, maxAge time.Duration) error {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	now := time.Now()
	cleaned := 0

	for hash, venv := range m.venvCache {
		if now.Sub(venv.LastUsedAt) > maxAge {
			// 删除虚拟环境目录
			if err := os.RemoveAll(venv.Path); err != nil {
				g.Log().Errorf(ctx, "删除虚拟环境失败: %v", err)
				continue
			}

			delete(m.venvCache, hash)
			cleaned++
			g.Log().Infof(ctx, "[虚拟环境] 清理: %s (未使用 %v)", hash[:8], now.Sub(venv.LastUsedAt))
		}
	}

	if cleaned > 0 {
		g.Log().Infof(ctx, "[虚拟环境] 清理完成: 删除 %d 个环境", cleaned)
	}

	return nil
}

// GetStats 获取统计信息
func (m *VenvManager) GetStats() map[string]interface{} {
	m.cacheMutex.RLock()
	defer m.cacheMutex.RUnlock()

	return map[string]interface{}{
		"total_venvs": len(m.venvCache),
		"base_dir":    m.baseDir,
	}
}
