package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/game-server/controller/internal/node"
	"github.com/game-server/controller/pkg/config"
	"go.uber.org/zap"
)

// FileHandler handles REST API requests for file operations
type FileHandler struct {
	nodeMgr *node.Manager
	cfg     *config.Config
	logger  *zap.Logger
}

// NewFileHandler creates a new file handler
func NewFileHandler(
	nodeMgr *node.Manager,
	cfg *config.Config,
	logger *zap.Logger,
) *FileHandler {
	return &FileHandler{
		nodeMgr: nodeMgr,
		cfg:     cfg,
		logger:  logger,
	}
}

// RegisterRoutes registers the file routes
func (h *FileHandler) RegisterRoutes(router *gin.RouterGroup) {
	files := router.Group("/nodes/:id/files")
	{
		files.GET("", h.ListFiles)
		files.GET("/exists", h.FileExists)
		files.POST("/directory", h.CreateDirectory)
		files.POST("/file", h.CreateFile)
		files.DELETE("", h.DeleteFile)
		files.PUT("/rename", h.RenameFile)
		files.PUT("/move", h.MoveFile)
		files.PUT("/copy", h.CopyFile)
		files.GET("/read", h.ReadFile)
		files.POST("/write", h.WriteFile)
		files.POST("/mkdir", h.Mkdir)
		files.POST("/zip", h.ZipFiles)
		files.POST("/unzip", h.UnzipFiles)
		files.POST("/upload", h.UploadFolder)
		files.GET("/download", h.DownloadFolder)
	}
}

// FileInfo represents file information for JSON response
type FileInfo struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsDirectory  bool   `json:"is_directory"`
	Size         int64  `json:"size"`
	ModifiedTime int64  `json:"modified_time"`
	CreatedTime  int64  `json:"created_time"`
	Permissions  string `json:"permissions"`
}

// ListFiles lists files in a directory
func (h *FileHandler) ListFiles(c *gin.Context) {
	nodeID := c.Param("id")
	path := c.Query("path")
	recursive := c.Query("recursive") == "true"

	if path == "" {
		path = "/"
	}

	result, err := h.nodeMgr.ListFiles(c.Request.Context(), nodeID, path, recursive)
	if err != nil {
		// Check if the context was canceled (client disconnected)
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("File listing canceled by client", zap.String("node_id", nodeID))
			return // Don't send response, client is gone
		}
		h.logger.Error("Failed to list files", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"files":        result.Files,
		"current_path": result.CurrentPath,
	})
}

// CreateDirectory creates a new directory
func (h *FileHandler) CreateDirectory(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	err := h.nodeMgr.CreateFile(c.Request.Context(), nodeID, req.Path, true)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Create directory canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to create directory", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Directory created successfully",
	})
}

// CreateFile creates a new file
func (h *FileHandler) CreateFile(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	err := h.nodeMgr.CreateFile(c.Request.Context(), nodeID, req.Path, false)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Create file canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to create file", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File created successfully",
	})
}

// DeleteFile deletes a file or directory
func (h *FileHandler) DeleteFile(c *gin.Context) {
	nodeID := c.Param("id")
	path := c.Query("path")
	recursive := c.Query("recursive") == "true"

	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Path is required",
		})
		return
	}

	err := h.nodeMgr.DeleteFile(c.Request.Context(), nodeID, path, recursive)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Delete file canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to delete file", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File deleted successfully",
	})
}

// RenameFile renames a file or directory
func (h *FileHandler) RenameFile(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		OldPath string `json:"old_path" binding:"required"`
		NewPath string `json:"new_path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	err := h.nodeMgr.RenameFile(c.Request.Context(), nodeID, req.OldPath, req.NewPath)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Rename file canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to rename file", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File renamed successfully",
	})
}

// MoveFile moves a file or directory
func (h *FileHandler) MoveFile(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		SourcePath string `json:"source_path" binding:"required"`
		DestPath   string `json:"dest_path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	err := h.nodeMgr.MoveFile(c.Request.Context(), nodeID, req.SourcePath, req.DestPath)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Move file canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to move file", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File moved successfully",
	})
}

// CopyFile copies a file or directory
func (h *FileHandler) CopyFile(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		SourcePath string `json:"source_path" binding:"required"`
		DestPath   string `json:"dest_path" binding:"required"`
		Recursive  bool   `json:"recursive"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	err := h.nodeMgr.CopyFile(c.Request.Context(), nodeID, req.SourcePath, req.DestPath, req.Recursive)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Copy file canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to copy file", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File copied successfully",
	})
}

// ReadFile reads file content
func (h *FileHandler) ReadFile(c *gin.Context) {
	nodeID := c.Param("id")
	path := c.Query("path")

	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Path is required",
		})
		return
	}

	result, err := h.nodeMgr.ReadFile(c.Request.Context(), nodeID, path)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Read file canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to read file", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"content":    result.Content,
		"total_size": result.TotalSize,
	})
}

// WriteFile writes content to a file
func (h *FileHandler) WriteFile(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		Path    string `json:"path" binding:"required"`
		Content []byte `json:"content"`
		Append  bool   `json:"append"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	err := h.nodeMgr.WriteFile(c.Request.Context(), nodeID, req.Path, req.Content, req.Append)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Write file canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to write file", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File written successfully",
	})
}

// FileExists checks if a file exists
func (h *FileHandler) FileExists(c *gin.Context) {
	nodeID := c.Param("id")
	path := c.Query("path")

	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Path is required",
		})
		return
	}

	result, err := h.nodeMgr.FileExists(c.Request.Context(), nodeID, path)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("File exists check canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to check file existence", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"exists":      result.Exists,
		"is_directory": result.IsDirectory,
	})
}

// Mkdir creates a directory with optional parent creation
func (h *FileHandler) Mkdir(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		Path    string `json:"path" binding:"required"`
		Parents bool   `json:"parents"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	err := h.nodeMgr.Mkdir(c.Request.Context(), nodeID, req.Path, req.Parents)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Mkdir canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to create directory", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Directory created successfully",
	})
}

// ZipFiles zips files or folders
func (h *FileHandler) ZipFiles(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		SourcePath string `json:"source_path" binding:"required"`
		DestPath   string `json:"dest_path" binding:"required"`
		Recursive  bool   `json:"recursive"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	err := h.nodeMgr.ZipFiles(c.Request.Context(), nodeID, req.SourcePath, req.DestPath, req.Recursive)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Zip files canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to zip files", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Files zipped successfully",
	})
}

// UnzipFiles unzips an archive
func (h *FileHandler) UnzipFiles(c *gin.Context) {
	nodeID := c.Param("id")

	var req struct {
		SourcePath string `json:"source_path" binding:"required"`
		DestPath   string `json:"dest_path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request: " + err.Error(),
		})
		return
	}

	err := h.nodeMgr.UnzipFiles(c.Request.Context(), nodeID, req.SourcePath, req.DestPath)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Unzip files canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to unzip files", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Files unzipped successfully",
	})
}

// UploadFolder handles folder upload (receives zip file and extracts)
func (h *FileHandler) UploadFolder(c *gin.Context) {
	nodeID := c.Param("id")
	destPath := c.Query("dest_path")

	if destPath == "" {
		destPath = "/"
	}

	// Get the uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No file uploaded: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// Read file content
	content := make([]byte, header.Size)
	_, err = file.Read(content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to read file: " + err.Error(),
		})
		return
	}

	// Upload and extract
	err = h.nodeMgr.UploadFolder(c.Request.Context(), nodeID, destPath, header.Filename, content)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Upload folder canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to upload folder", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Folder uploaded successfully",
	})
}

// DownloadFolder handles folder download (zips and sends)
func (h *FileHandler) DownloadFolder(c *gin.Context) {
	nodeID := c.Param("id")
	path := c.Query("path")

	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Path is required",
		})
		return
	}

	// Download and zip the folder
	result, err := h.nodeMgr.DownloadFolder(c.Request.Context(), nodeID, path)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debug("Download folder canceled by client", zap.String("node_id", nodeID))
			return
		}
		h.logger.Error("Failed to download folder", zap.Error(err), zap.String("node_id", nodeID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Set headers for download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+result.Filename)
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")

	c.Data(http.StatusOK, "application/zip", result.Content)
}
