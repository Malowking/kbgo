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
    image_urls: List[str] = Field(default=[], description="该块包含的图片URL列表")


class ParseResponse(BaseModel):
    """文件解析响应模型"""

    success: bool = Field(True, description="是否成功")
    result: List[ChunkData] = Field(..., description="解析结果列表")
    total_image_urls: List[str] = Field(default=[], description="所有图片URL列表")
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