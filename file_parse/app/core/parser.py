"""
文件解析模块
负责将各种格式的文件转换为Markdown文本，并提取图片
"""
import os
import uuid
import re
import subprocess
import tempfile
from pathlib import Path
from typing import Tuple, List, Optional
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

        # 用于追踪需要清理的临时文件
        temp_files_to_cleanup = []

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
                # 如果生成了修改后的PDF，添加到清理列表
                if modified_file_path != file_path:
                    temp_files_to_cleanup.append(modified_file_path)
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

        finally:
            # 清理临时文件
            for temp_file in temp_files_to_cleanup:
                try:
                    if os.path.exists(temp_file):
                        os.unlink(temp_file)
                        logger.info(f"Cleaned up temporary file: {temp_file}")
                except Exception as e:
                    logger.warning(f"Failed to cleanup temporary file {temp_file}: {str(e)}")

    def _save_image_as_jpeg(self, img: Image.Image, save_path: Path) -> None:
        """
        将PIL Image对象转换并保存为JPEG格式

        Args:
            img: PIL Image对象
            save_path: 保存路径
        """
        # 如果是RGBA模式，转换为RGB（JPEG不支持透明度）
        if img.mode in ('RGBA', 'LA', 'P'):
            # 创建白色背景
            rgb_img = Image.new('RGB', img.size, (255, 255, 255))
            if img.mode == 'P':
                img = img.convert('RGBA')
            rgb_img.paste(img, mask=img.split()[-1] if img.mode in ('RGBA', 'LA') else None)
            img = rgb_img
        elif img.mode != 'RGB':
            img = img.convert('RGB')

        # 缩放并保存为JPEG
        img.thumbnail(settings.MAX_IMAGE_SIZE, Image.Resampling.LANCZOS)
        img.save(save_path, 'JPEG', quality=85, optimize=True)

    def _convert_emf_wmf_to_jpeg(self, img_data: bytes, img_file: str) -> Optional[Image.Image]:
        """
        尝试将EMF/WMF格式图片转换为PIL Image对象

        Args:
            img_data: 图片数据
            img_file: 图片文件名（用于检测格式）

        Returns:
            PIL Image对象，如果转换失败则返回None
        """
        # 检查是否是EMF/WMF格式
        is_emf_wmf = img_file.lower().endswith(('.emf', '.wmf'))

        if not is_emf_wmf:
            return None

        try:
            # 方法1: 尝试使用LibreOffice转换（如果安装了的话）
            import shutil
            soffice_path = shutil.which('soffice') or shutil.which('libreoffice')

            if soffice_path:
                logger.info(f"Found LibreOffice at: {soffice_path}, attempting to convert {img_file}")

                # 创建输出目录（不使用with，手动管理）
                tmp_out_dir = tempfile.mkdtemp()
                tmp_in = None

                try:
                    # 保存临时输入文件，使用正确的扩展名
                    suffix = '.emf' if img_file.lower().endswith('.emf') else '.wmf'
                    tmp_in = tempfile.NamedTemporaryFile(suffix=suffix, delete=False)
                    tmp_in.write(img_data)
                    tmp_in.close()
                    tmp_in_path = tmp_in.name

                    logger.info(f"Saved temp EMF/WMF file to: {tmp_in_path}")

                    # 使用LibreOffice转换为PNG
                    cmd = [
                        soffice_path,
                        '--headless',
                        '--convert-to', 'png',
                        '--outdir', tmp_out_dir,
                        tmp_in_path
                    ]

                    logger.info(f"Running LibreOffice command: {' '.join(cmd)}")
                    result = subprocess.run(cmd, capture_output=True, timeout=30, text=True)

                    logger.info(f"LibreOffice exit code: {result.returncode}")
                    if result.stdout:
                        logger.info(f"LibreOffice stdout: {result.stdout}")
                    if result.stderr:
                        logger.warning(f"LibreOffice stderr: {result.stderr}")

                    # 查找转换后的PNG文件
                    png_file = Path(tmp_out_dir) / f"{Path(tmp_in_path).stem}.png"
                    logger.info(f"Looking for converted PNG at: {png_file}")

                    # 列出输出目录中的所有文件
                    output_files = list(Path(tmp_out_dir).glob('*'))
                    logger.info(f"Files in output directory: {output_files}")

                    if png_file.exists():
                        img = Image.open(png_file)
                        logger.info(f"Successfully converted {img_file} using LibreOffice")
                        result_img = img.copy()  # 复制一份，因为临时文件会被删除
                        return result_img
                    else:
                        logger.warning(f"PNG file not found after LibreOffice conversion: {png_file}")

                except Exception as e:
                    logger.error(f"LibreOffice conversion error: {e}", exc_info=True)
                finally:
                    # 清理临时文件
                    try:
                        if tmp_in and os.path.exists(tmp_in.name):
                            os.unlink(tmp_in.name)
                        import shutil as shutil_clean
                        if os.path.exists(tmp_out_dir):
                            shutil_clean.rmtree(tmp_out_dir)
                    except Exception as cleanup_error:
                        logger.warning(f"Cleanup error: {cleanup_error}")

        except Exception as e:
            logger.debug(f"EMF/WMF conversion attempt failed: {e}")

        return None

    def _extract_images_from_docx(self, file_path: str, format_url: bool = True) -> List[str]:
        """从DOCX文件中提取图片并统一转换为JPEG格式"""
        image_urls = []
        try:
            with ZipFile(file_path, 'r') as docx:
                # 获取所有图片文件
                image_files = [f for f in docx.namelist() if f.startswith('word/media/')]

                for img_file in image_files:
                    # 读取图片数据
                    img_data = docx.read(img_file)

                    # 生成唯一文件名，统一使用.jpeg扩展名
                    file_name = f"{uuid.uuid4().hex}.jpeg"
                    save_path = self.image_dir / file_name

                    try:
                        # 先检查是否是EMF/WMF格式，如果是则直接转换
                        if img_file.lower().endswith(('.emf', '.wmf')):
                            logger.warning(f"Detected EMF/WMF format image: {img_file}")

                            # 直接使用LibreOffice转换
                            img = self._convert_emf_wmf_to_jpeg(img_data, img_file)

                            if img is None:
                                # 转换失败，保存原始格式
                                logger.warning(f"Failed to convert {img_file}: LibreOffice not found or conversion failed. "
                                             f"Install LibreOffice to enable EMF/WMF conversion: "
                                             f"'brew install --cask libreoffice' (macOS) or "
                                             f"'sudo apt-get install libreoffice' (Ubuntu/Debian)")
                                # 保存原始格式
                                original_ext = Path(img_file).suffix or '.dat'
                                file_name = f"{uuid.uuid4().hex}{original_ext}"
                                save_path = self.image_dir / file_name
                                save_path.write_bytes(img_data)

                                # 跳过后续处理，直接记录URL
                                if format_url:
                                    url = f"{settings.base_url}/images/{file_name}"
                                else:
                                    url = f"image/{file_name}"
                                image_urls.append(url)
                                logger.info(f"Extracted EMF/WMF image (original format): {file_name}")
                                continue
                            else:
                                logger.info(f"Successfully converted {img_file} using LibreOffice")
                        else:
                            # 非EMF/WMF格式，使用PIL正常打开
                            img = Image.open(BytesIO(img_data))

                        # 使用统一的方法转换并保存为JPEG
                        self._save_image_as_jpeg(img, save_path)
                        logger.info(f"Successfully converted {img_file} to JPEG")

                    except Exception as e:
                        # 如果所有方法都失败，保存原始数据并使用原始扩展名
                        logger.warning(f"Failed to convert image {img_file} to JPEG: {e}, saving as original")
                        # 使用原始扩展名
                        original_ext = Path(img_file).suffix or '.dat'
                        file_name = f"{uuid.uuid4().hex}{original_ext}"
                        save_path = self.image_dir / file_name
                        save_path.write_bytes(img_data)

                    # 根据 format_url 参数决定返回格式
                    if format_url:
                        # 返回完整URL：http://127.0.0.1:8002/images/xxx.jpeg
                        url = f"{settings.base_url}/images/{file_name}"
                    else:
                        # 返回相对路径：image/xxx.jpeg（用于Go服务拼接）
                        url = f"image/{file_name}"
                    image_urls.append(url)
                    logger.info(f"Extracted image from DOCX: {file_name}")

        except Exception as e:
            logger.error(f"Error extracting images from DOCX: {e}")

        return image_urls

    def _extract_and_replace_pdf_images(self, file_path: str, format_url: bool = True) -> Tuple[List[str], str]:
        """
        从PDF中提取图片并统一转换为JPEG格式，在原位置用路径文本替换图片，生成修改后的PDF

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

                    # 生成唯一文件名，统一使用.jpeg扩展名
                    file_name = f"{uuid.uuid4().hex}.jpeg"
                    save_path = self.image_dir / file_name

                    try:
                        # 尝试用PIL打开图片并转换为JPEG
                        img_obj = Image.open(BytesIO(img_data))

                        # 使用统一的方法转换并保存为JPEG
                        self._save_image_as_jpeg(img_obj, save_path)

                    except Exception as e:
                        # 如果PIL无法处理，尝试直接保存原始数据
                        logger.warning(f"Failed to convert PDF image to JPEG: {e}, saving as original")
                        save_path.write_bytes(img_data)

                    # 根据 format_url 参数决定返回格式
                    if format_url:
                        # 返回完整URL：http://127.0.0.1:8002/images/xxx.jpeg
                        url = f"{settings.base_url}/images/{file_name}"
                    else:
                        # 返回相对路径：image/xxx.jpeg（用于Go服务拼接）
                        url = f"image/{file_name}"
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
                    logger.info(f"Extracted and converted image from PDF: {file_name}")

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
        """从PPTX文件中提取图片并统一转换为JPEG格式"""
        image_urls = []
        try:
            with ZipFile(file_path, 'r') as pptx:
                # 获取所有图片文件
                image_files = [f for f in pptx.namelist() if f.startswith('ppt/media/')]

                for img_file in image_files:
                    # 读取图片数据
                    img_data = pptx.read(img_file)

                    # 生成唯一文件名，统一使用.jpeg扩展名
                    file_name = f"{uuid.uuid4().hex}.jpeg"
                    save_path = self.image_dir / file_name

                    try:
                        # 尝试用PIL打开图片并转换为JPEG
                        img = Image.open(BytesIO(img_data))

                        # 使用统一的方法转换并保存为JPEG
                        self._save_image_as_jpeg(img, save_path)

                    except Exception as e:
                        # 如果PIL无法处理，尝试直接保存原始数据
                        logger.warning(f"Failed to convert PPTX image {img_file} to JPEG: {e}, saving as original")
                        save_path.write_bytes(img_data)

                    # 根据 format_url 参数决定返回格式
                    if format_url:
                        # 返回完整URL：http://127.0.0.1:8002/images/xxx.jpeg
                        url = f"{settings.base_url}/images/{file_name}"
                    else:
                        # 返回相对路径：image/xxx.jpeg（用于Go服务拼接）
                        url = f"image/{file_name}"
                    image_urls.append(url)
                    logger.info(f"Extracted and converted image from PPTX: {file_name}")

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