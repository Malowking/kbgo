package cmd

import (
	"mime"
	"net/http"
	"reflect"
	"strings"

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gmeta"
)

const (
	contentTypeEventStream  = "text/event-stream"
	contentTypeOctetStream  = "application/octet-stream"
	contentTypeMixedReplace = "multipart/x-mixed-replace"
)

const (
	// 知识库上传文件大小限制: 500MB
	maxKnowledgeBaseUploadSize = 500 << 20 // 500MB
	// 文件对话上传文件大小限制: 5MB
	maxChatFileUploadSize = 5 << 20 // 5MB
)

var (
	// streamContentType is the content types for stream response.
	streamContentType = []string{contentTypeEventStream, contentTypeOctetStream, contentTypeMixedReplace}
)

// MiddlewareMultipartMaxMemory 根据不同的路由设置不同的文件上传大小限制
func MiddlewareMultipartMaxMemory(r *ghttp.Request) {
	// 获取请求路径
	path := r.URL.Path

	// 只处理 multipart/form-data 请求
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		r.Middleware.Next()
		return
	}

	// 根据不同路由设置不同的上传限制
	if strings.HasPrefix(path, "/api/v1/upload") {
		// 知识库上传: 500MB
		if err := r.ParseMultipartForm(maxKnowledgeBaseUploadSize); err != nil {
			r.Response.WriteStatus(http.StatusRequestEntityTooLarge)
			r.Response.WriteJson(ghttp.DefaultHandlerResponse{
				Code:    gcode.CodeInvalidParameter.Code(),
				Message: "File size exceeds the knowledge base upload limit (500MB)",
				Data:    nil,
			})
			return
		}
	} else if strings.HasPrefix(path, "/api/v1/chat") {
		// 文件对话: 5MB
		if err := r.ParseMultipartForm(maxChatFileUploadSize); err != nil {
			r.Response.WriteStatus(http.StatusRequestEntityTooLarge)
			r.Response.WriteJson(ghttp.DefaultHandlerResponse{
				Code:    gcode.CodeInvalidParameter.Code(),
				Message: "File size exceeds the chat upload limit (5MB)",
				Data:    nil,
			})
			return
		}
	}

	r.Middleware.Next()
}

// MiddlewareHandlerResponse is the default middleware handling handler response object and its error.
func MiddlewareHandlerResponse(r *ghttp.Request) {
	r.Middleware.Next()

	// There's custom buffer content, it then exits current handler.
	if r.Response.BufferLength() > 0 || r.Response.Writer.BytesWritten() > 0 {
		return
	}

	// It does not output common response content if it is stream response.
	mediaType, _, _ := mime.ParseMediaType(r.Response.Header().Get("Content-Type"))
	for _, ct := range streamContentType {
		if mediaType == ct {
			return
		}
	}

	var (
		msg  string
		err  = r.GetError()
		res  = r.GetHandlerResponse()
		code = gerror.Code(err)
	)
	if err != nil {
		if code == gcode.CodeNil {
			code = gcode.CodeInternalError
		}
		msg = err.Error()
	} else {
		if r.Response.Status > 0 && r.Response.Status != http.StatusOK {
			switch r.Response.Status {
			case http.StatusNotFound:
				code = gcode.CodeNotFound
			case http.StatusForbidden:
				code = gcode.CodeNotAuthorized
			default:
				code = gcode.CodeUnknown
			}
			// It creates an error as it can be retrieved by other middlewares.
			err = gerror.NewCode(code, msg)
			r.SetError(err)
		} else {
			code = gcode.CodeOK
		}
		msg = code.Message()
	}
	if noWrapResp(r) {
		r.Response.WriteJson(res)
		return
	}
	r.Response.WriteJson(ghttp.DefaultHandlerResponse{
		Code:    code.Code(),
		Message: msg,
		Data:    res,
	})
}

// 中间件中判断
func noWrapResp(r *ghttp.Request) bool {
	handler := r.GetServeHandler().Handler
	if handler.Info.Type != nil && handler.Info.Type.NumIn() == 2 {
		var objectReq = reflect.New(handler.Info.Type.In(1))
		if v := gmeta.Get(objectReq, "no_wrap_resp"); !v.IsEmpty() {
			return v.Bool()
		}
	}
	return false
}
