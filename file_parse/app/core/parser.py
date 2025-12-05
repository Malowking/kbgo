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

logger = get_logger("file_parse")


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

    def parse(self, file_path: str, format_url: bool = True) -> Tuple[str, List[str]]:
        """
        解析文件为Markdown文本,并提取图片

        Args:
            file_path: 文件路径
            format_url: 是否格式化为静态地址URL，False则返回绝对路径

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
            modified_file_path = file_path  # 用于Markitdown解析的文件路径

            if ext == '.docx':
                image_urls = self._extract_images_from_docx(file_path, format_url)
            elif ext == '.pdf':
                # PDF需要先处理：提取图片、替换为路径、生成修改后的PDF
                image_urls, modified_file_path = self._extract_and_replace_pdf_images(file_path, format_url)
            elif ext == '.pptx':
                image_urls = self._extract_images_from_pptx(file_path, format_url)

            # 解析为 Markdown
            result = self.md.convert(modified_file_path)
            md_text = result.text_content or ""

            # 删除base64格式的图片（Markitdown可能生成的）
            md_text = self._remove_base64_images(md_text)

            # 替换图片占位符为实际URL（如果有的话）
            if image_urls:
                md_text = self._replace_image_placeholders(md_text, image_urls)

            logger.info(f"Successfully parsed file: {file_path}, length: {len(md_text)}, images: {len(image_urls)}")
            return md_text, image_urls

        except Exception as e:
            logger.error(f"Error parsing file {file_path}: {str(e)}")
            raise RuntimeError(f"Error converting file: {str(e)}") from e

    def _extract_images_from_docx(self, file_path: str, format_url: bool = True) -> List[str]:
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

                    # 根据 format_url 参数决定返回格式
                    if format_url:
                        url = f"{settings.base_url}/images/{file_name}"
                    else:
                        url = str(save_path.absolute())
                    image_urls.append(url)
                    logger.info(f"Extracted image from DOCX: {file_name}")

        except Exception as e:
            logger.error(f"Error extracting images from DOCX: {e}")

        return image_urls

    def _extract_and_replace_pdf_images(self, file_path: str, format_url: bool = True) -> Tuple[List[str], str]:
        """
        从PDF中提取图片，并在原位置用路径文本替换图片，生成修改后的PDF

        Args:
            file_path: 原始PDF文件路径
            format_url: 是否格式化为静态地址URL

        Returns:
            (图片URL列表, 修改后的PDF文件路径)
        """
        image_urls = []
        images_to_delete = {}

        try:
            doc = fitz.open(file_path)

            # 第一步：提取所有图片并记录位置
            for page_num in range(len(doc)):
                page = doc[page_num]
                images = page.get_images()

                if page_num not in images_to_delete:
                    images_to_delete[page_num] = []

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
                        img_obj = Image.open(BytesIO(img_data))
                        img_obj.thumbnail(settings.MAX_IMAGE_SIZE, Image.Resampling.LANCZOS)
                        img_obj.save(save_path)
                    else:
                        save_path.write_bytes(img_data)

                    # 根据 format_url 参数决定返回格式
                    if format_url:
                        url = f"{settings.base_url}/images/{file_name}"
                    else:
                        url = str(save_path.absolute())
                    image_urls.append(url)

                    # 获取图片在页面上的位置
                    image_rects = page.get_image_rects(xref)

                    # 在图片位置插入路径文本
                    for rect in image_rects:
                        # 先用白色矩形覆盖图片区域
                        page.draw_rect(rect, color=(1, 1, 1), fill=(1, 1, 1), width=0)

                        # 在原图片位置插入路径文本
                        page.insert_textbox(
                            rect,
                            url,
                            fontsize=8,
                            color=(0, 0, 0),
                            fill=(1, 1, 1),
                            align=fitz.TEXT_ALIGN_LEFT
                        )

                    # 记录要删除的图片
                    images_to_delete[page_num].append(xref)
                    logger.info(f"Extracted and marked image from PDF: {file_name}")

            # 第二步：删除所有图片对象
            for page_num, xrefs in images_to_delete.items():
                page = doc[page_num]
                for xref in xrefs:
                    page.delete_image(xref)

            # 第三步：保存修改后的PDF
            modified_pdf_name = f"modified_{uuid.uuid4().hex}.pdf"
            modified_pdf_path = self.image_dir / modified_pdf_name
            doc.save(str(modified_pdf_path))
            doc.close()

            logger.info(f"Created modified PDF with {len(image_urls)} images replaced: {modified_pdf_path}")
            return image_urls, str(modified_pdf_path)

        except Exception as e:
            logger.error(f"Error extracting and replacing PDF images: {e}")
            # 如果出错，返回原文件路径
            return [], file_path

    def _extract_images_from_pptx(self, file_path: str, format_url: bool = True) -> List[str]:
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

                    # 根据 format_url 参数决定返回格式
                    if format_url:
                        url = f"{settings.base_url}/images/{file_name}"
                    else:
                        url = str(save_path.absolute())
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

    def _remove_base64_images(self, md_text: str) -> str:
        """
        删除Markdown中的base64格式图片

        删除类似 ![](data:image/png;base64,...) 或 ![alt text](data:image/jpeg;base64,...)
        这种Markitdown可能生成的base64图片标记
        """
        # 匹配完整的base64图片语法：![任意文本](data:image/类型;base64,实际数据)
        pattern = r'!\[.*?\]\(data:image/[^;]+;base64,[^\)]*\)'

        # 删除所有匹配的base64图片
        cleaned_text = re.sub(pattern, '', md_text)

        # 清理可能产生的多余空行
        cleaned_text = re.sub(r'\n{3,}', '\n\n', cleaned_text)

        return cleaned_text

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