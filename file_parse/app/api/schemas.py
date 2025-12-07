"""
API 数据模型
定义请求和响应的数据结构
"""
from typing import List, Optional
from pydantic import BaseModel, Field, validator

from app.config import settings


class ParseRequest(BaseModel):
    """文件解析请求模型"""

    file_path: str = Field(..., description="要解析的文件路径")
    chunk_size: int = Field(
        default=settings.DEFAULT_CHUNK_SIZE,
        description="文本块大小，-1 表示不切分，全量返回"
    )
    chunk_overlap: int = Field(
        default=settings.DEFAULT_CHUNK_OVERLAP,
        ge=0,
        description="文本块重叠大小"
    )
    separators: List[str] = Field(
        default=settings.DEFAULT_SEPARATORS,
        description="文本分隔符列表"
    )
    image_url_format: bool = Field(
        default=True,
        description="是否格式化图片URL为静态地址，False则返回图片绝对路径"
    )

    @validator("separators")
    def decode_separators(cls, v):
        """解码分隔符中的转义字符"""
        if not v:
            return v

        # 转义字符映射
        escape_map = {
            '\\n': '\n',   # 换行符
            '\\r': '\r',   # 回车符
            '\\t': '\t',   # 制表符
            '\\\\': '\\',  # 反斜杠
        }

        decoded = []
        for sep in v:
            # 替换常见的转义序列
            decoded_sep = sep
            for escape_seq, real_char in escape_map.items():
                decoded_sep = decoded_sep.replace(escape_seq, real_char)
            decoded.append(decoded_sep)

        return decoded

    @validator("chunk_size")
    def validate_chunk_size(cls, v):
        """验证chunk_size，允许-1（不切分）或正常范围内的值"""
        if v == -1:
            return v
        if v < settings.MIN_CHUNK_SIZE or v > settings.MAX_CHUNK_SIZE:
            raise ValueError(
                f"chunk_size must be -1 (no chunking) or between "
                f"{settings.MIN_CHUNK_SIZE} and {settings.MAX_CHUNK_SIZE}"
            )
        return v

    @validator("chunk_overlap")
    def validate_chunk_overlap(cls, v, values):
        """验证chunk_overlap必须小于chunk_size"""
        chunk_size = values.get("chunk_size", settings.DEFAULT_CHUNK_SIZE)
        # 如果不切分，overlap无意义
        if chunk_size == -1:
            return 0
        if v >= chunk_size:
            raise ValueError("chunk_overlap must be less than chunk_size")
        return v


class ChunkData(BaseModel):
    """文本块数据模型"""

    chunk_index: int = Field(..., description="文本块索引")
    text: str = Field(..., description="文本内容")


class ParseResponse(BaseModel):
    """文件解析响应模型"""

    success: bool = Field(True, description="是否成功")
    result: List[ChunkData] = Field(..., description="解析结果列表")
    image_urls: List[str] = Field(default=[], description="所有图片URL列表")
    total_chunks: int = Field(..., description="总文本块数量")
    total_images: int = Field(..., description="总图片数量")
    file_info: dict = Field(default={}, description="文件信息")


class HealthResponse(BaseModel):
    """健康检查响应模型"""

    status: str = Field("healthy", description="服务状态")
    message: str = Field("File Parse Service is running", description="状态消息")
    version: str = Field(settings.APP_VERSION, description="服务版本")


class ErrorResponse(BaseModel):
    """错误响应模型"""

    success: bool = Field(False, description="是否成功")
    error: str = Field(..., description="错误类型")
    detail: str = Field(..., description="错误详情")
    code: int = Field(..., description="错误代码")


class OCRRequest(BaseModel):
    """OCR识别请求模型"""

    image_path: str = Field(..., description="要识别的图片路径")
    auto_detect_lang: bool = Field(
        default=True,
        description="是否自动检测简繁体，True则自动检测，False则使用lang参数"
    )
    lang: Optional[str] = Field(
        default=None,
        description="指定语言模型。支持多种格式：'ch'/'ch_sim'(简体), 'chinese_cht'/'ch_tra'(繁体)，会自动转换为对应引擎的格式"
    )

    @validator("lang")
    def validate_lang(cls, v):
        """验证语言参数"""
        if v is not None and v not in ['ch', 'ch_sim', 'chinese_cht', 'ch_tra', 'simplified', 'traditional']:
            raise ValueError("lang must be one of: 'ch', 'ch_sim', 'chinese_cht', 'ch_tra', 'simplified', 'traditional'")
        return v


class OCRResult(BaseModel):
    """OCR识别结果"""

    text: str = Field(..., description="识别的文本")
    confidence: float = Field(..., description="置信度")
    box: List[List[float]] = Field(..., description="文字框坐标")


class OCRResponse(BaseModel):
    """OCR识别响应模型"""

    success: bool = Field(True, description="是否成功")
    markdown: str = Field(..., description="Markdown格式的识别结果")
    language: str = Field(..., description="使用的语言模型")
    total_lines: int = Field(..., description="识别的文本行数")
    raw_results: List[OCRResult] = Field(default=[], description="原始OCR识别结果")
    image_info: dict = Field(default={}, description="图片信息")