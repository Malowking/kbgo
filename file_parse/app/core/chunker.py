"""
文本分块模块
负责将长文本按照指定规则分割成多个块
"""
import re
from typing import List

from app.config import settings
from app.utils import get_logger

logger = get_logger("file_parse")


class TextChunker:
    """文本分块器"""

    def __init__(
        self,
        chunk_size: int = None,
        chunk_overlap: int = None,
        separators: List[str] = None
    ):
        """
        初始化分块器

        Args:
            chunk_size: 每个块的大小（字符数），-1 表示不切分
            chunk_overlap: 块之间的重叠大小
            separators: 分隔符列表，按优先级排序
        """
        self.chunk_size = chunk_size if chunk_size is not None else settings.DEFAULT_CHUNK_SIZE
        self.chunk_overlap = chunk_overlap if chunk_overlap is not None else settings.DEFAULT_CHUNK_OVERLAP
        self.separators = separators or settings.DEFAULT_SEPARATORS

        # 验证参数
        if self.chunk_size != -1 and self.chunk_size <= 0:
            raise ValueError("chunk_size must be -1 (no chunking) or greater than 0")
        if self.chunk_size != -1 and self.chunk_overlap >= self.chunk_size:
            raise ValueError("chunk_overlap must be less than chunk_size")

        # 编译图片URL的正则表达式
        # Markdown 格式: ![alt](http://...)
        self.img_pattern = re.compile(r"!\[.*?\]\(http.*?\)")
        # 普通 URL 格式: http://... 或 https://...
        self.url_pattern = re.compile(r'https?://[^\s\)]+')
        # 绝对路径格式: /path/to/file.ext 或 C:\path\to\file.ext
        self.path_pattern = re.compile(r'(?:^|[\s\(])(/[^\s\)]+\.[a-zA-Z0-9]+|[A-Za-z]:\\[^\s\)]+\.[a-zA-Z0-9]+)')

    def chunk(self, text: str) -> List[str]:
        """
        将文本分割成多个块

        Args:
            text: 要分割的文本

        Returns:
            文本块列表
        """
        text_len = len(text)

        # 如果 chunk_size 为 -1，不切分，全量返回
        if self.chunk_size == -1:
            logger.info(f"chunk_size is -1, returning full text without chunking (length: {text_len})")
            return [text]

        # 如果文本短于chunk_size，直接返回
        if text_len <= self.chunk_size:
            logger.info(f"Text length ({text_len}) <= chunk_size, returning as single chunk")
            return [text]

        chunks = []
        start = 0
        max_iterations = text_len + 1000  # 防止无限循环的最大迭代次数
        iteration_count = 0

        while start < text_len:
            iteration_count += 1
            if iteration_count > max_iterations:
                logger.error(f"Chunking exceeded max iterations ({max_iterations}), breaking to prevent infinite loop")
                break

            # 计算初始结束位置
            end = min(start + self.chunk_size, text_len)

            # 如果 end 没有前进，强制前进
            if end <= start:
                logger.warning(f"end ({end}) <= start ({start}), forcing progress")
                end = start + 1

            chunk = text[start:end]

            # 尝试在分隔符处切分
            chunk, end = self._adjust_to_separator(text, start, end, chunk)

            # 避免切断图片URL
            chunk, end = self._avoid_splitting_images(text, start, end, chunk)

            # 再次确保 end 有效
            if end <= start:
                logger.warning(f"After adjustments, end ({end}) <= start ({start}), forcing end = start + 1")
                end = start + 1
                chunk = text[start:end]

            chunks.append(chunk)

            # 计算下一个块的起始位置（考虑重叠）
            new_start = end - self.chunk_overlap

            # 防止死循环：确保 start 至少前进
            # 如果 overlap 太大导致不前进，则强制前进
            if new_start <= start:
                # 尝试前进至少 chunk_size 的 10%，或者 1 个字符
                min_progress = max(1, int(self.chunk_size * 0.1))
                new_start = start + min_progress
                logger.debug(f"Forcing progress: start {start} -> {new_start} (min_progress={min_progress})")

            start = new_start
            if start >= text_len:
                break

        logger.info(f"Split text into {len(chunks)} chunks (iterations: {iteration_count})")
        return chunks

    def _adjust_to_separator(
        self,
        text: str,
        start: int,
        end: int,
        chunk: str
    ) -> tuple:
        """
        调整chunk边界到最近的分隔符

        Args:
            text: 原始文本
            start: 当前chunk起始位置
            end: 当前chunk结束位置
            chunk: 当前chunk内容

        Returns:
            (调整后的chunk, 调整后的end位置)
        """
        # 如果已经到达文本末尾，不需要调整
        if end >= len(text):
            return chunk, end

        # 如果 chunk 太短，不调整
        if len(chunk) < 10:
            return chunk, end

        # 按优先级尝试每个分隔符
        min_chunk_length = max(10, int(self.chunk_size * 0.3))  # 至少保留 30% 或 10 个字符

        for sep in self.separators:
            if not sep:
                continue

            idx = chunk.rfind(sep)
            # 只有在找到分隔符且位置合理时才调整
            if idx != -1 and idx >= min_chunk_length:
                new_end = start + idx + len(sep)
                # 确保新的 end 比 start 大，且向前推进了
                if new_end > start and new_end <= end:
                    end = new_end
                    chunk = text[start:end]
                    break

        return chunk, end

    def _avoid_splitting_images(
        self,
        text: str,
        start: int,
        end: int,
        chunk: str
    ) -> tuple:
        """
        避免切断图片URL和路径

        保护以下格式不被切分：
        1. Markdown 图片格式: ![alt](http://...)
        2. 普通 URL: http://... 或 https://...
        3. 绝对路径: /path/to/file.ext 或 C:\path\to\file.ext

        Args:
            text: 原始文本
            start: 当前chunk起始位置
            end: 当前chunk结束位置
            chunk: 当前chunk内容

        Returns:
            (调整后的chunk, 调整后的end位置)
        """
        # 1. 检查 Markdown 格式的图片是否被截断
        # 例如: ![alt](http://...  <-- 这里被截断了
        img_start_pattern = r'!\[[^\]]*\]\([^\)]*$'
        incomplete_match = re.search(img_start_pattern, chunk)

        if incomplete_match:
            # 图片 URL 被截断，需要扩展 chunk 直到找到完整的图片
            remaining_text = text[end:]
            # 查找下一个 ) 来闭合图片
            close_paren = remaining_text.find(')')

            if close_paren != -1:
                # 找到了闭合括号，扩展 chunk
                extend_length = close_paren + 1
                new_end = min(end + extend_length, len(text))
                chunk = text[start:new_end]
                end = new_end
                logger.debug(f"Extended chunk to include complete Markdown image: end={end}")
                return chunk, end

        # 2. 检查普通 URL 是否被截断
        # 在 chunk 末尾查找可能被截断的 URL
        # 向后查看最多100个字符，看是否有URL开始
        check_length = min(100, len(chunk))
        chunk_tail = chunk[-check_length:] if check_length > 0 else chunk

        # 查找所有 URL 匹配
        url_matches = list(self.url_pattern.finditer(chunk_tail))

        if url_matches:
            # 获取最后一个匹配的 URL
            last_url_match = url_matches[-1]
            url_start_in_tail = last_url_match.start()
            url_end_in_tail = last_url_match.end()

            # 检查这个 URL 是否在 chunk 的末尾（可能被截断）
            # 如果 URL 在chunk末尾的5个字符内，可能被截断
            if url_end_in_tail >= len(chunk_tail) - 5:
                # URL 可能被截断，尝试扩展
                remaining_text = text[end:]
                # 继续匹配URL的剩余部分（非空白字符且不是括号）
                extended_match = re.match(r'[^\s\)]+', remaining_text)

                if extended_match:
                    extend_length = extended_match.end()
                    new_end = min(end + extend_length, len(text))
                    chunk = text[start:new_end]
                    end = new_end
                    logger.debug(f"Extended chunk to include complete URL: end={end}")
                    return chunk, end

        # 3. 检查绝对路径是否被截断
        # 查找可能被截断的路径
        # 路径模式: /path/to/file 或 C:\path\to\file
        path_patterns = [
            r'(/[^\s\)]+)$',  # Unix路径在末尾
            r'([A-Za-z]:\\[^\s\)]+)$'  # Windows路径在末尾
        ]

        for pattern in path_patterns:
            path_match = re.search(pattern, chunk)
            if path_match:
                # 找到了可能被截断的路径
                path_start = path_match.start(1)
                remaining_text = text[end:]

                # 继续匹配路径的剩余部分
                # 路径可能包含: 字母、数字、下划线、连字符、点、斜杠
                extended_match = re.match(r'[a-zA-Z0-9_\-\./\\]+', remaining_text)

                if extended_match:
                    extend_length = extended_match.end()
                    new_end = min(end + extend_length, len(text))
                    chunk = text[start:new_end]
                    end = new_end
                    logger.debug(f"Extended chunk to include complete path: end={end}")
                    return chunk, end

        return chunk, end


def create_chunker(
    chunk_size: int = None,
    chunk_overlap: int = None,
    separators: List[str] = None
) -> TextChunker:
    """
    创建文本分块器（工厂函数）

    Args:
        chunk_size: 每个块的大小
        chunk_overlap: 块之间的重叠大小
        separators: 分隔符列表

    Returns:
        TextChunker实例
    """
    return TextChunker(chunk_size, chunk_overlap, separators)