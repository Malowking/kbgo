"""
文件解析模块
负责将各种格式的文件转换为Markdown文本，并提取图片
"""
import os
import uuid
import re
from pathlib import Path
from typing import Tuple, List
from zipfile import ZipFile
from io import BytesIO

from markitdown import MarkItDown
from PIL import Image
import fitz  # PyMuPDF

from app.config import settings
from app.utils import get_logger

logger = get_logger("parser")


class FileParser:
    """文件解析器"""

    def __init__(self):
        """
        初始化文件解析器

        Args:
        """
        self.md = MarkItDown()
        self.image_dir = Path(settings.IMAGE_DIR)
        logger.info("FileParser initialized")

    def parse(self, file_path: str) -> Tuple[str, List[str]]:
        """
        解析文件为Markdown文本，并提取图片

        Args:
            file_path: 文件路径

        Returns:
            (Markdown格式的文本内容, 图片URL列表)

        Raises:
            FileNotFoundError: 文件不存在
            ValueError: 文件格式不支持
            RuntimeError: 解析过程出错
        """
        # 检查文件是否存在
        if not os.path.isabs(file_path):
            file_path = os.path.abspath(file_path)

        if not os.path.exists(file_path):
            raise FileNotFoundError(f"File not found: {file_path}")

        # 检查文件扩展名
        ext = Path(file_path).suffix.lower()
        if ext not in settings.SUPPORTED_FORMATS:
            logger.warning(f"Unsupported file format: {ext}")

        try:
            logger.info(f"Parsing file: {file_path}")

            # 提取图片
            image_urls = []
            if self.extract_images:
                if ext == '.docx':
                    image_urls = self._extract_images_from_docx(file_path)
                elif ext == '.pdf':
                    image_urls = self._extract_images_from_pdf(file_path)
                elif ext == '.pptx':
                    image_urls = self._extract_images_from_pptx(file_path)

            # 解析为 Markdown
            result = self.md.convert(file_path)
            md_text = result.text_content or ""

            # 替换图片占位符为实际URL
            if image_urls:
                md_text = self._replace_image_placeholders(md_text, image_urls)

            logger.info(f"Successfully parsed file: {file_path}, length: {len(md_text)}, images: {len(image_urls)}")
            return md_text, image_urls

        except Exception as e:
            logger.error(f"Error parsing file {file_path}: {str(e)}")
            raise RuntimeError(f"Error converting file: {str(e)}") from e

    def _extract_images_from_docx(self, file_path: str) -> List[str]:
        """从DOCX文件中提取图片"""
        image_urls = []
        try:
            with ZipFile(file_path, 'r') as docx:
                # 获取所有图片文件
                image_files = [f for f in docx.namelist() if f.startswith('word/media/')]

                for img_file in image_files:
                    # 读取图片数据
                    img_data = docx.read(img_file)

                    # 生成唯一文件名
                    ext = Path(img_file).suffix or '.png'
                    file_name = f"{uuid.uuid4().hex}{ext}"
                    save_path = self.image_dir / file_name

                    # 保存图片（可选：压缩）
                    if ext.lower() in ['.png', '.jpg', '.jpeg', '.webp']:
                        img = Image.open(BytesIO(img_data))
                        img.thumbnail(settings.MAX_IMAGE_SIZE, Image.Resampling.LANCZOS)
                        img.save(save_path)
                    else:
                        save_path.write_bytes(img_data)

                    # 生成URL
                    url = f"{settings.base_url}/images/{file_name}"
                    image_urls.append(url)
                    logger.info(f"Extracted image from DOCX: {file_name}")

        except Exception as e:
            logger.error(f"Error extracting images from DOCX: {e}")

        return image_urls

    def _extract_images_from_pdf(self, file_path: str) -> List[str]:
        """从PDF文件中提取图片"""
        image_urls = []
        try:
            doc = fitz.open(file_path)
            for page_num in range(len(doc)):
                page = doc[page_num]
                images = page.get_images()

                for img_index, img in enumerate(images):
                    xref = img[0]
                    base_image = doc.extract_image(xref)
                    img_data = base_image["image"]
                    img_ext = base_image["ext"]

                    # 生成唯一文件名
                    file_name = f"{uuid.uuid4().hex}.{img_ext}"
                    save_path = self.image_dir / file_name

                    # 保存图片
                    if img_ext in ['png', 'jpg', 'jpeg', 'webp']:
                        img = Image.open(BytesIO(img_data))
                        img.thumbnail(settings.MAX_IMAGE_SIZE, Image.Resampling.LANCZOS)
                        img.save(save_path)
                    else:
                        save_path.write_bytes(img_data)

                    # 生成URL
                    url = f"{settings.base_url}/images/{file_name}"
                    image_urls.append(url)
                    logger.info(f"Extracted image from PDF: {file_name}")

            doc.close()
        except Exception as e:
            logger.error(f"Error extracting images from PDF: {e}")

        return image_urls

    def _extract_images_from_pptx(self, file_path: str) -> List[str]:
        """从PPTX文件中提取图片"""
        image_urls = []
        try:
            with ZipFile(file_path, 'r') as pptx:
                # 获取所有图片文件
                image_files = [f for f in pptx.namelist() if f.startswith('ppt/media/')]

                for img_file in image_files:
                    # 读取图片数据
                    img_data = pptx.read(img_file)

                    # 生成唯一文件名
                    ext = Path(img_file).suffix or '.png'
                    file_name = f"{uuid.uuid4().hex}{ext}"
                    save_path = self.image_dir / file_name

                    # 保存图片
                    if ext.lower() in ['.png', '.jpg', '.jpeg', '.webp']:
                        img = Image.open(BytesIO(img_data))
                        img.thumbnail(settings.MAX_IMAGE_SIZE, Image.Resampling.LANCZOS)
                        img.save(save_path)
                    else:
                        save_path.write_bytes(img_data)

                    # 生成URL
                    url = f"{settings.base_url}/images/{file_name}"
                    image_urls.append(url)
                    logger.info(f"Extracted image from PPTX: {file_name}")

        except Exception as e:
            logger.error(f"Error extracting images from PPTX: {e}")

        return image_urls

    def _replace_image_placeholders(self, md_text: str, image_urls: List[str]) -> str:
        """替换Markdown中的图片占位符为实际URL"""
        # 匹配 ![](data:image/png;base64...) 或 ![](data:image/jpeg;base64...)
        pattern = r'!\[(.*?)\]\(data:image/[^;]+;base64\.\.\.\)'

        url_index = 0
        def replacer(match):
            nonlocal url_index
            if url_index < len(image_urls):
                alt_text = match.group(1) or f"image-{url_index}"
                url = image_urls[url_index]
                url_index += 1
                return f"![{alt_text}]({url})"
            return match.group(0)

        md_text = re.sub(pattern, replacer, md_text)
        return md_text

    def parse_from_bytes(self, content: bytes, file_extension: str) -> Tuple[str, List[str]]:
        """
        从字节内容解析文件

        Args:
            content: 文件内容（字节）
            file_extension: 文件扩展名（如 .pdf, .docx）

        Returns:
            (Markdown格式的文本内容, 图片URL列表)

        Raises:
            ValueError: 文件格式不支持
            RuntimeError: 解析过程出错
        """
        import tempfile

        # 创建临时文件
        with tempfile.NamedTemporaryFile(suffix=file_extension, delete=False) as tmp:
            tmp.write(content)
            tmp_path = tmp.name

        try:
            return self.parse(tmp_path)
        finally:
            # 清理临时文件
            try:
                os.unlink(tmp_path)
            except Exception as e:
                logger.warning(f"Failed to delete temp file {tmp_path}: {str(e)}")


def create_parser() -> FileParser:
    """
    创建文件解析器（工厂函数）

    Args:

    Returns:
        FileParser实例
    """
    return FileParser()