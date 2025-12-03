"""
日志配置模块
提供统一的日志记录功能
"""
import logging
import sys
from logging.handlers import RotatingFileHandler
from pathlib import Path
from typing import Optional

from app.config import settings


def setup_logger(
    name: str,
    log_file: Optional[Path] = None,
    level: str = None
) -> logging.Logger:
    """
    配置并返回logger实例

    Args:
        name: logger名称
        log_file: 日志文件路径，如果为None则只输出到控制台
        level: 日志级别，默认使用配置中的级别

    Returns:
        配置好的logger实例
    """
    logger = logging.getLogger(name)

    # 如果logger已经配置过，直接返回
    if logger.handlers:
        return logger

    # 设置日志级别
    log_level = getattr(logging, level or settings.LOG_LEVEL)
    logger.setLevel(log_level)

    # 创建格式化器
    formatter = logging.Formatter(settings.LOG_FORMAT)

    # 添加控制台处理器
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setLevel(log_level)
    console_handler.setFormatter(formatter)
    logger.addHandler(console_handler)

    # 如果指定了日志文件，添加文件处理器
    if log_file:
        # 使用 'w' 模式在每次启动时覆盖日志文件
        file_handler = logging.FileHandler(log_file, mode='w', encoding='utf-8')
        file_handler.setLevel(log_level)
        file_handler.setFormatter(formatter)
        logger.addHandler(file_handler)

    # 防止日志向上传播
    logger.propagate = False

    return logger


def get_logger(name: str = "file_parse") -> logging.Logger:
    """
    获取logger实例（便捷方法）
    所有logger共享同一个日志文件

    Args:
        name: logger名称

    Returns:
        logger实例
    """
    log_file = settings.LOG_DIR / "file_parse.log"
    return setup_logger(name, log_file)


# 创建默认logger
default_logger = get_logger()