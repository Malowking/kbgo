"""
API 路由定义
"""
import os
from pathlib import Path
from typing import List

from fastapi import APIRouter, HTTPException, status
from fastapi.responses import JSONResponse

from app.api.schemas import (
    ParseRequest,
    ParseResponse,
    ChunkData,
    HealthResponse,
    ErrorResponse
)
from app.core import create_parser, create_chunker, image_handler
from app.config import settings
from app.utils import get_logger

logger = get_logger("file_parse")
router = APIRouter()


@router.get("/", response_model=HealthResponse)
async def root():
    """根路径 - 服务状态"""
    return HealthResponse()


@router.get("/health", response_model=HealthResponse)
async def health_check():
    """健康检查接口"""
    return HealthResponse()


@router.post("/parse", response_model=ParseResponse)
async def parse_file(request: ParseRequest):
    """
    解析文件接口

    将指定路径的文件解析为带分块的Markdown文本，支持图片提取和URL替换
    """
    try:
        # 验证文件路径
        file_path = request.file_path
        if not os.path.isabs(file_path):
            file_path = os.path.abspath(file_path)

        if not os.path.exists(file_path):
            raise HTTPException(
                status_code=status.HTTP_404_NOT_FOUND,
                detail=f"File not found: {file_path}"
            )

        # 获取文件信息
        file_stat = os.stat(file_path)
        file_info = {
            "name": Path(file_path).name,
            "size": file_stat.st_size,
            "extension": Path(file_path).suffix.lower(),
            "path": file_path
        }

        logger.info(f"Processing file: {file_info['name']} ({file_info['size']} bytes)")

        # 1. 解析文件为Markdown文本并提取图片
        parser = create_parser()
        md_text, extracted_image_urls = parser.parse(file_path)

        # 2. 替换base64图片为URL（如果有markitdown生成的base64图片）
        md_text, base64_image_urls = await image_handler.replace_images_with_urls(md_text)

        # 合并所有图片URL
        all_image_urls = extracted_image_urls + base64_image_urls

        # 3. 文本分块
        chunker = create_chunker(
            chunk_size=request.chunk_size,
            chunk_overlap=request.chunk_overlap,
            separators=request.separators
        )
        chunks = chunker.chunk(md_text)

        # 4. 处理图片去重并生成结果
        chunk_results = image_handler.remove_duplicate_images(chunks)

        # 5. 收集所有唯一图片URL
        unique_images = set(all_image_urls)
        for chunk_data in chunk_results:
            unique_images.update(chunk_data["image_urls"])

        # 6. 构建响应数据
        response_chunks = [
            ChunkData(
                chunk_index=chunk["chunk_index"],
                text=chunk["text"],
                image_urls=chunk["image_urls"]
            )
            for chunk in chunk_results
        ]

        response = ParseResponse(
            result=response_chunks,
            total_image_urls=list(unique_images),
            total_chunks=len(response_chunks),
            total_images=len(unique_images),
            file_info=file_info
        )

        logger.info(
            f"Successfully parsed {file_info['name']}: "
            f"{response.total_chunks} chunks, {response.total_images} images"
        )

        return response

    except HTTPException:
        # 重新抛出HTTP异常
        raise
    except ValueError as e:
        logger.error(f"Validation error: {str(e)}")
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=str(e)
        )
    except Exception as e:
        logger.error(f"Internal server error: {str(e)}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Internal server error: {str(e)}"
        )


@router.get("/supported-formats")
async def get_supported_formats():
    """获取支持的文件格式列表"""
    return {
        "supported_formats": settings.SUPPORTED_FORMATS,
        "description": "List of supported file formats for parsing"
    }


@router.get("/config")
async def get_config():
    """获取当前配置信息"""
    return {
        "chunk_size_range": {
            "min": settings.MIN_CHUNK_SIZE,
            "max": settings.MAX_CHUNK_SIZE,
            "default": settings.DEFAULT_CHUNK_SIZE,
            "no_chunking": -1,
            "description": "Set chunk_size to -1 to return full document without chunking"
        },
        "default_chunk_overlap": settings.DEFAULT_CHUNK_OVERLAP,
        "default_separators": settings.DEFAULT_SEPARATORS,
        "max_image_size": settings.MAX_IMAGE_SIZE,
        "supported_formats": settings.SUPPORTED_FORMATS
    }