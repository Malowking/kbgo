"""
OCR识别服务
自动检测并使用可用的OCR引擎（PaddleOCR或EasyOCR）
"""
import os
import sys
from typing import List, Dict, Any, Optional, Tuple
from pathlib import Path

from PIL import Image

from app.core.chinese_detector import ChineseDetector
from app.utils import get_logger

logger = get_logger("ocr_service")


class OCRService:
    """OCR识别服务 - 自动适配多种OCR引擎"""

    def __init__(self):
        """初始化OCR服务"""
        self._reader_ch = None  # 简体中文OCR实例
        self._reader_cht = None  # 繁体中文OCR实例
        self._chinese_detector = ChineseDetector()
        self._engine = None  # OCR引擎类型: 'paddle' 或 'easy'
        self._detect_engine()

    def _detect_engine(self):
        """检测可用的OCR引擎"""
        # 优先尝试PaddleOCR
        try:
            import paddleocr
            self._engine = 'paddle'
            logger.info("OCR Engine: PaddleOCR detected and will be used")
            return
        except ImportError:
            pass

        # 降级使用EasyOCR
        try:
            import easyocr
            self._engine = 'easy'
            logger.info("OCR Engine: EasyOCR detected and will be used")
            return
        except ImportError:
            pass

        # 没有可用的OCR引擎
        raise ImportError(
            "No OCR engine available. Please install either:\n"
            "  - PaddleOCR: pip install paddleocr (for Linux)\n"
            "  - EasyOCR: pip install easyocr (for macOS/Windows)\n"
            "Or run: poetry install"
        )

    def _get_reader(self, lang: str):
        """
        获取OCR Reader实例（懒加载）

        Args:
            lang: 语言类型，根据引擎不同有不同的值
                - PaddleOCR: 'ch' (简体), 'chinese_cht' (繁体)
                - EasyOCR: 'ch_sim' (简体), 'ch_tra' (繁体)

        Returns:
            OCR Reader实例
        """
        if self._engine == 'paddle':
            return self._get_paddle_reader(lang)
        elif self._engine == 'easy':
            return self._get_easy_reader(lang)
        else:
            raise RuntimeError("No OCR engine available")

    def _get_paddle_reader(self, lang: str):
        """获取PaddleOCR实例"""
        from paddleocr import PaddleOCR

        if lang == 'chinese_cht':
            if self._reader_cht is None:
                logger.info("Initializing PaddleOCR Traditional Chinese model...")
                self._reader_cht = PaddleOCR(
                    use_angle_cls=True,
                    lang='chinese_cht',
                    show_log=False
                )
                logger.info("PaddleOCR Traditional Chinese model loaded")
            return self._reader_cht
        else:
            if self._reader_ch is None:
                logger.info("Initializing PaddleOCR Simplified Chinese model...")
                self._reader_ch = PaddleOCR(
                    use_angle_cls=True,
                    lang='ch',
                    show_log=False
                )
                logger.info("PaddleOCR Simplified Chinese model loaded")
            return self._reader_ch

    def _get_easy_reader(self, lang: str):
        """获取EasyOCR实例"""
        import easyocr

        if lang == 'ch_tra':
            if self._reader_cht is None:
                logger.info("Initializing EasyOCR Traditional Chinese model...")
                self._reader_cht = easyocr.Reader(
                    ['ch_tra', 'en'],
                    gpu=False,
                    verbose=False
                )
                logger.info("EasyOCR Traditional Chinese model loaded")
            return self._reader_cht
        else:
            if self._reader_ch is None:
                logger.info("Initializing EasyOCR Simplified Chinese model...")
                self._reader_ch = easyocr.Reader(
                    ['ch_sim', 'en'],
                    gpu=False,
                    verbose=False
                )
                logger.info("EasyOCR Simplified Chinese model loaded")
            return self._reader_ch

    def _normalize_lang(self, lang: Optional[str], is_traditional: bool = False) -> str:
        """
        标准化语言代码为对应引擎的格式

        Args:
            lang: 输入的语言代码（可能为None）
            is_traditional: 是否为繁体

        Returns:
            标准化后的语言代码
        """
        # 如果已经指定了语言代码，转换为对应引擎的格式
        if lang:
            if self._engine == 'paddle':
                if lang in ['ch_tra', 'chinese_cht', 'traditional']:
                    return 'chinese_cht'
                else:
                    return 'ch'
            else:  # easy
                if lang in ['ch', 'chinese_cht', 'traditional']:
                    return 'ch_tra' if lang in ['chinese_cht', 'traditional'] else 'ch_sim'
                elif lang in ['ch_tra', 'ch_sim']:
                    return lang
                else:
                    return 'ch_sim'

        # 根据is_traditional参数生成语言代码
        if self._engine == 'paddle':
            return 'chinese_cht' if is_traditional else 'ch'
        else:  # easy
            return 'ch_tra' if is_traditional else 'ch_sim'

    def get_engine_info(self) -> Dict[str, str]:
        """获取当前OCR引擎信息"""
        info = {
            'engine': self._engine,
            'engine_name': 'PaddleOCR' if self._engine == 'paddle' else 'EasyOCR',
            'platform': sys.platform
        }

        if self._engine == 'paddle':
            info['lang_simplified'] = 'ch'
            info['lang_traditional'] = 'chinese_cht'
        else:
            info['lang_simplified'] = 'ch_sim'
            info['lang_traditional'] = 'ch_tra'

        return info

    def detect_language(self, image_path: str, sample_size: int = 100) -> Tuple[str, float]:
        """
        检测图片中的文字是简体还是繁体

        Args:
            image_path: 图片路径
            sample_size: 采样文本字符数（用于快速检测）

        Returns:
            (语言代码, 繁体字占比)
        """
        try:
            # 使用简体中文模型快速识别部分内容
            lang_code = self._normalize_lang(None, is_traditional=False)
            reader = self._get_reader(lang_code)

            # 根据引擎类型调用不同的识别方法
            if self._engine == 'paddle':
                result = reader.ocr(image_path, cls=True)
                if not result or not result[0]:
                    logger.warning(f"No text detected in image: {image_path}")
                    return lang_code, 0.0

                # 提取文本用于检测
                sample_text = ''
                for line in result[0][:5]:  # 只取前5行
                    if line and len(line) >= 2:
                        sample_text += line[1][0]
                        if len(sample_text) >= sample_size:
                            break
            else:  # easy
                result = reader.readtext(image_path)
                if not result:
                    logger.warning(f"No text detected in image: {image_path}")
                    return lang_code, 0.0

                # 提取文本用于检测
                sample_text = ''
                for detection in result:
                    sample_text += detection[1]
                    if len(sample_text) >= sample_size:
                        break

            logger.info(f"Sample text for detection: {sample_text[:50]}...")

            # 检测简繁体
            chinese_type, ratio = self._chinese_detector.detect_chinese_type(sample_text)

            # 返回标准化的语言代码
            is_traditional = (chinese_type == 'traditional')
            detected_lang = self._normalize_lang(None, is_traditional=is_traditional)
            logger.info(f"Detected language: {detected_lang}, traditional ratio: {ratio:.2%}")

            return detected_lang, ratio

        except Exception as e:
            logger.error(f"Error detecting language: {str(e)}", exc_info=True)
            return self._normalize_lang(None, is_traditional=False), 0.0

    def recognize_image(
        self,
        image_path: str,
        auto_detect_lang: bool = True,
        lang: Optional[str] = None
    ) -> List[Dict[str, Any]]:
        """
        识别图片中的文字

        Args:
            image_path: 图片路径
            auto_detect_lang: 是否自动检测语言
            lang: 指定语言（会自动转换为对应引擎的格式）

        Returns:
            识别结果列表，每个元素包含：
            - text: 识别的文本
            - confidence: 置信度
            - box: 文字位置坐标
        """
        try:
            # 验证图片文件存在
            if not os.path.exists(image_path):
                raise FileNotFoundError(f"Image file not found: {image_path}")

            # 确定使用的语言模型
            if lang is None and auto_detect_lang:
                detected_lang, ratio = self.detect_language(image_path)
                lang = detected_lang
                logger.info(f"Auto-detected language: {lang}")
            elif lang is None:
                lang = self._normalize_lang(None, is_traditional=False)
            else:
                # 标准化用户输入的语言代码
                lang = self._normalize_lang(lang)

            # 获取对应的OCR Reader
            reader = self._get_reader(lang)

            # 执行OCR识别
            logger.info(f"Recognizing image: {image_path} with {self._engine} engine, lang={lang}")

            formatted_results = []

            if self._engine == 'paddle':
                result = reader.ocr(image_path, cls=True)
                if not result or not result[0]:
                    logger.warning(f"No text detected in image: {image_path}")
                    return []

                for line in result[0]:
                    if line and len(line) >= 2:
                        box = line[0]  # 文字框坐标
                        text_info = line[1]  # (文本, 置信度)
                        formatted_results.append({
                            'text': text_info[0],
                            'confidence': float(text_info[1]),
                            'box': [[float(x), float(y)] for x, y in box]
                        })
            else:  # easy
                result = reader.readtext(image_path)
                if not result:
                    logger.warning(f"No text detected in image: {image_path}")
                    return []

                for detection in result:
                    box = detection[0]  # 文字框坐标
                    text = detection[1]  # 识别的文本
                    confidence = detection[2]  # 置信度
                    formatted_results.append({
                        'text': text,
                        'confidence': float(confidence),
                        'box': [[float(x), float(y)] for x, y in box]
                    })

            logger.info(f"Successfully recognized {len(formatted_results)} text regions")
            return formatted_results

        except Exception as e:
            logger.error(f"Error recognizing image: {str(e)}", exc_info=True)
            raise

    def format_as_markdown(self, ocr_results: List[Dict[str, Any]]) -> str:
        """
        将OCR识别结果格式化为Markdown

        Args:
            ocr_results: OCR识别结果

        Returns:
            Markdown格式的文本
        """
        if not ocr_results:
            return ""

        # 按照文字位置（从上到下，从左到右）排序
        sorted_results = sorted(ocr_results, key=lambda x: (x['box'][0][1], x['box'][0][0]))

        # 构建markdown文本
        lines = []
        prev_y = None
        current_line_texts = []

        for result in sorted_results:
            text = result['text']
            y_coord = result['box'][0][1]

            # 判断是否为同一行（y坐标差异小于阈值）
            if prev_y is None or abs(y_coord - prev_y) < 20:  # 20像素的阈值
                current_line_texts.append(text)
                prev_y = y_coord
            else:
                # 新的一行，保存之前的行
                if current_line_texts:
                    lines.append(' '.join(current_line_texts))
                current_line_texts = [text]
                prev_y = y_coord

        # 添加最后一行
        if current_line_texts:
            lines.append(' '.join(current_line_texts))

        # 组合成段落（连续的行合并为一个段落）
        paragraphs = []
        current_paragraph = []

        for line in lines:
            line = line.strip()
            if not line:
                # 空行，结束当前段落
                if current_paragraph:
                    paragraphs.append('\n'.join(current_paragraph))
                    current_paragraph = []
            else:
                current_paragraph.append(line)

        # 添加最后一个段落
        if current_paragraph:
            paragraphs.append('\n'.join(current_paragraph))

        # 用两个换行符分隔段落
        markdown_text = '\n\n'.join(paragraphs)

        return markdown_text

    def recognize_and_format(
        self,
        image_path: str,
        auto_detect_lang: bool = True,
        lang: Optional[str] = None
    ) -> Dict[str, Any]:
        """
        识别图片并格式化为Markdown

        Args:
            image_path: 图片路径
            auto_detect_lang: 是否自动检测语言
            lang: 指定语言

        Returns:
            包含识别结果的字典：
            - markdown: Markdown格式的文本
            - raw_results: 原始OCR结果
            - language: 使用的语言模型
            - engine: 使用的OCR引擎
            - total_lines: 识别的文本行数
        """
        try:
            # 执行OCR识别
            if lang is None and auto_detect_lang:
                detected_lang, _ = self.detect_language(image_path)
                lang = detected_lang

            ocr_results = self.recognize_image(image_path, auto_detect_lang=False, lang=lang)

            # 格式化为Markdown
            markdown_text = self.format_as_markdown(ocr_results)

            return {
                'markdown': markdown_text,
                'raw_results': ocr_results,
                'language': lang if lang else self._normalize_lang(None, False),
                'engine': self._engine,
                'total_lines': len(ocr_results)
            }

        except Exception as e:
            logger.error(f"Error in recognize_and_format: {str(e)}", exc_info=True)
            raise


# 创建全局OCR服务实例
ocr_service = OCRService()