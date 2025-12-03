"""
图片处理模块
负责处理base64图片、保存图片、替换图片URL等
"""
import re
import uuid
import base64
import asyncio
from io import BytesIO
from pathlib import Path
from typing import Tuple, List

from PIL import Image

from app.config import settings
from app.utils import get_logger

logger = get_logger("image_handler")


class ImageHandler:
    """图片处理器"""

    def __init__(self):
        self.image_dir = settings.IMAGE_DIR
        self.max_size = settings.MAX_IMAGE_SIZE
        self.base_url = settings.base_url

    async def save_base64_image(self, base64_str: str) -> str:
        """
        保存base64编码的图片到本地并返回URL

        Args:
            base64_str: base64编码的图片字符串

        Returns:
            图片的URL地址

        Raises:
            ValueError: 如果base64字符串格式无效
        """
        # 解析base64字符串
        match = re.match(r"data:(image/.*?);base64,(.*)", base64_str, re.DOTALL)
        if not match:
            raise ValueError("Invalid base64 image string")

        mime_type, data_str = match.groups()
        img_bytes = base64.b64decode(data_str)

        # 确定文件扩展名
        ext_map = {
            "image/png": ".png",
            "image/jpeg": ".jpg",
            "image/jpg": ".jpg",
            "image/webp": ".webp",
            "image/svg+xml": ".svg"
        }
        ext = ext_map.get(mime_type, ".png")

        # 生成唯一文件名
        file_name = uuid.uuid4().hex
        file_path = self.image_dir / (file_name + ext)

        # 保存图片
        if ext == ".svg":
            # SVG直接写入文件
            await asyncio.to_thread(file_path.write_bytes, img_bytes)
        else:
            # 其他图片格式需要缩放
            img = Image.open(BytesIO(img_bytes))
            img.thumbnail(self.max_size, Image.LANCZOS)
            await asyncio.to_thread(img.save, file_path)

        logger.info(f"Saved image: {file_name}{ext}")
        return f"{self.base_url}/images/{file_name}{ext}"

    async def replace_images_with_urls(self, md_text: str) -> Tuple[str, List[str]]:
        """
        替换Markdown文本中的base64图片为URL

        Args:
            md_text: 包含base64图片的Markdown文本

        Returns:
            (替换后的文本, 图片URL列表)
        """
        pattern = r"!\[.*?\]\((data:image\/.*?;base64,.*?)\)"
        images = re.findall(pattern, md_text)

        if not images:
            return md_text, []

        # 并发保存所有图片
        tasks = [self.save_base64_image(img) for img in images]
        urls = await asyncio.gather(*tasks)

        # 替换文本中的base64为URL
        for img_data, url in zip(images, urls):
            md_text = md_text.replace(img_data, url, 1)

        logger.info(f"Replaced {len(urls)} images with URLs")
        return md_text, urls

    @staticmethod
    def extract_image_urls(text: str) -> List[str]:
        """
        从文本中提取所有图片URL

        Args:
            text: Markdown文本

        Returns:
            图片URL列表
        """
        pattern = r"!\[.*?\]\((http.*?)\)"
        return re.findall(pattern, text)

    @staticmethod
    def remove_duplicate_images(chunks: List[str]) -> List[dict]:
        """
        去重图片并为每个chunk关联图片URL

        Args:
            chunks: 文本块列表

        Returns:
            包含文本和图片URL的字典列表
        """
        seen_images = set()
        result = []

        for idx, chunk in enumerate(chunks):
            imgs_in_chunk = ImageHandler.extract_image_urls(chunk)
            new_imgs = [img for img in imgs_in_chunk if img not in seen_images]
            seen_images.update(new_imgs)

            # 删除重复图片的Markdown语法
            for img in imgs_in_chunk:
                if img not in new_imgs:
                    chunk = chunk.replace(f"![]({img})", "")

            result.append({
                "chunk_index": idx,
                "text": chunk,
                "image_urls": new_imgs
            })

        return result


# 创建全局图片处理器实例
image_handler = ImageHandler()