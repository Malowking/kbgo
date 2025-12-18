"""
FastAPI 应用入口
"""
from contextlib import asynccontextmanager
from fastapi import FastAPI, HTTPException
from fastapi.staticfiles import StaticFiles
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import JSONResponse

from app.api import router
from app.api.schemas import ErrorResponse
from app.config import settings
from app.utils import get_logger

logger = get_logger("file_parse")


@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    应用生命周期管理器

    在应用启动时执行初始化逻辑，在应用关闭时执行清理逻辑
    """
    # 启动逻辑
    logger.info(f"Starting {settings.APP_NAME} v{settings.APP_VERSION}")
    logger.info(f"Server running at {settings.base_url}")
    logger.info(f"Image directory: {settings.IMAGE_DIR}")
    logger.info(f"Log directory: {settings.LOG_DIR}")
    logger.info(f"Supported formats: {', '.join(settings.SUPPORTED_FORMATS)}")

    yield

    # 关闭逻辑
    logger.info(f"Shutting down {settings.APP_NAME}")


def create_app() -> FastAPI:
    """
    创建并配置 FastAPI 应用

    Returns:
        配置好的 FastAPI 应用实例
    """
    app = FastAPI(
        title=settings.APP_NAME,
        version=settings.APP_VERSION,
        description="A powerful file parsing service that converts documents to Markdown with chunking and image handling",
        docs_url="/docs",
        redoc_url="/redoc",
        lifespan=lifespan
    )

    # 配置 CORS 中间件
    app.add_middleware(
        CORSMiddleware,
        allow_origins=settings.CORS_ORIGINS,
        allow_credentials=settings.CORS_ALLOW_CREDENTIALS,
        allow_methods=settings.CORS_ALLOW_METHODS,
        allow_headers=settings.CORS_ALLOW_HEADERS,
    )

    # 挂载静态文件目录（用于访问保存的图片）
    app.mount("/images", StaticFiles(directory=str(settings.IMAGE_DIR)), name="images")

    # 注册路由
    app.include_router(router)

    # 注册异常处理器
    @app.exception_handler(HTTPException)
    async def http_exception_handler(request, exc: HTTPException):
        """HTTP异常处理器"""
        return JSONResponse(
            status_code=exc.status_code,
            content=ErrorResponse(
                error="HTTPException",
                detail=exc.detail,
                code=exc.status_code
            ).model_dump()
        )

    @app.exception_handler(Exception)
    async def general_exception_handler(request, exc: Exception):
        """通用异常处理器"""
        logger.error(f"Unhandled exception: {str(exc)}", exc_info=True)
        return JSONResponse(
            status_code=500,
            content=ErrorResponse(
                error="InternalServerError",
                detail="An internal server error occurred",
                code=500
            ).model_dump()
        )

    return app


# 创建应用实例
app = create_app()


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "app.main:app",
        host=settings.HOST,
        port=settings.PORT,
        reload=settings.DEBUG,
        log_level=settings.LOG_LEVEL.lower()
    )