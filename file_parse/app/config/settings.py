"""
配置管理模块
支持从环境变量和配置文件加载配置
.env 文件中的配置优先级高于默认值
"""
import sys
from pathlib import Path
from typing import Tuple
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """应用配置类"""

    # 服务配置
    APP_NAME: str = "File Parse Service"
    APP_VERSION: str = "1.0.0"
    HOST: str = "127.0.0.1"
    PORT: int = 8002
    DEBUG: bool = False

    # 超时配置（用于大文件处理，如30MB+的文档）
    TIMEOUT_KEEP_ALIVE: int = 600  # Keep-Alive超时时间（秒），默认600秒（10分钟）
    TIMEOUT_GRACEFUL_SHUTDOWN: int = 30  # 优雅关闭超时时间（秒）
    LIMIT_CONCURRENCY: int = 100  # 最大并发连接数
    BACKLOG: int = 100  # 最大排队连接数

    # 路径配置
    IMAGE_DIR: str = ""  # 图片存储目录，必须在 .env 中配置

    # 图片配置
    MAX_IMAGE_SIZE: Tuple[int, int] = (1024, 1024)
    ALLOWED_IMAGE_FORMATS: list = ["png", "jpg", "jpeg", "webp", "svg"]

    # 文件解析配置
    DEFAULT_CHUNK_SIZE: int = 1000
    DEFAULT_CHUNK_OVERLAP: int = 100
    DEFAULT_SEPARATORS: list = ["\n\n", "\n", " ", ""]
    MAX_CHUNK_SIZE: int = 100000
    MIN_CHUNK_SIZE: int = 100

    # 支持的文件格式
    SUPPORTED_FORMATS: list = [
        ".txt", ".md", ".pdf", ".docx", ".xlsx",
        ".pptx", ".html", ".htm", ".csv", ".json"
    ]

    # 日志配置
    LOG_LEVEL: str = "INFO"
    LOG_FORMAT: str = "%(asctime)s - %(name)s - %(levelname)s - %(message)s"
    LOG_MAX_BYTES: int = 10 * 1024 * 1024  # 10MB
    LOG_BACKUP_COUNT: int = 5

    # CORS 配置
    CORS_ORIGINS: list = ["*"]
    CORS_ALLOW_CREDENTIALS: bool = True
    CORS_ALLOW_METHODS: list = ["*"]
    CORS_ALLOW_HEADERS: list = ["*"]

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=True,
        extra="ignore"
    )

    @property
    def BASE_DIR(self) -> Path:
        """项目根目录"""
        return Path(__file__).parent.parent.parent

    @property
    def LOG_DIR(self) -> Path:
        """日志目录"""
        return self.BASE_DIR / "logs"

    @property
    def base_url(self) -> str:
        """获取服务基础URL"""
        return f"http://{self.HOST}:{self.PORT}"

    def validate_and_setup(self):
        """验证配置并创建必要的目录"""
        # 检查 IMAGE_DIR 是否配置
        if not self.IMAGE_DIR:
            print("错误: IMAGE_DIR 未配置")
            print("请在 .env 文件中设置 IMAGE_DIR 为图片存储的绝对路径")
            print("示例: IMAGE_DIR=/Users/your-name/images")
            sys.exit(1)

        # 转换为 Path 对象并验证
        image_path = Path(self.IMAGE_DIR)
        if not image_path.is_absolute():
            print(f"错误: IMAGE_DIR 必须是绝对路径，当前值: {self.IMAGE_DIR}")
            sys.exit(1)

        # 创建图片目录
        try:
            image_path.mkdir(parents=True, exist_ok=True)
            self.IMAGE_DIR = str(image_path)
            print(f"✓ 图片存储目录: {self.IMAGE_DIR}")
        except Exception as e:
            print(f"错误: 无法创建图片目录 {image_path}: {e}")
            sys.exit(1)

        # 创建日志目录
        try:
            self.LOG_DIR.mkdir(parents=True, exist_ok=True)
            print(f"✓ 日志目录: {self.LOG_DIR}")
        except Exception as e:
            print(f"错误: 无法创建日志目录 {self.LOG_DIR}: {e}")
            sys.exit(1)


# 创建全局配置实例
settings = Settings()
settings.validate_and_setup()