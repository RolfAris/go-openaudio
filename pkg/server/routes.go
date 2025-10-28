package server

import (
	"github.com/labstack/echo/v4"
	"go.akshayshah.org/connectproto"
	"google.golang.org/protobuf/encoding/protojson"

	corev1connect "github.com/OpenAudio/go-openaudio/pkg/api/core/v1/v1connect"
	ethv1connect "github.com/OpenAudio/go-openaudio/pkg/api/eth/v1/v1connect"
	storagev1connect "github.com/OpenAudio/go-openaudio/pkg/api/storage/v1/v1connect"
	systemv1connect "github.com/OpenAudio/go-openaudio/pkg/api/system/v1/v1connect"
)

// Common global options
var (
	marshalOpts   = protojson.MarshalOptions{EmitUnpopulated: true}
	unmarshalOpts = protojson.UnmarshalOptions{DiscardUnknown: true}

	// Compose them into the Connect handler option
	connectJSONOpt = connectproto.WithJSON(marshalOpts, unmarshalOpts)
)

func (app *Server) RegisterRoutes(e *echo.Echo) {
	core := app.core
	storage := app.storage
	system := app.system
	eth := app.eth

	// ConnectRPC Routes
	rpcGroup := e.Group("")
	corePath, coreHandler := corev1connect.NewCoreServiceHandler(core, connectJSONOpt)
	rpcGroup.POST(corePath+"*", echo.WrapHandler(coreHandler))
	rpcGroup.GET(corePath+"*", echo.WrapHandler(coreHandler))

	storagePath, storageHandler := storagev1connect.NewStorageServiceHandler(storage, connectJSONOpt)
	rpcGroup.POST(storagePath+"*", echo.WrapHandler(storageHandler))
	rpcGroup.GET(storagePath+"*", echo.WrapHandler(storageHandler))

	systemPath, systemHandler := systemv1connect.NewSystemServiceHandler(system, connectJSONOpt)
	rpcGroup.POST(systemPath+"*", echo.WrapHandler(systemHandler))
	rpcGroup.GET(systemPath+"*", echo.WrapHandler(systemHandler))

	ethPath, ethHandler := ethv1connect.NewEthServiceHandler(eth, connectJSONOpt)
	rpcGroup.POST(ethPath+"*", echo.WrapHandler(ethHandler))
	rpcGroup.GET(ethPath+"*", echo.WrapHandler(ethHandler))

	// REST Routes
}
