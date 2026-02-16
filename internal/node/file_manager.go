package node

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// File operation result types

// FileListResult represents the result of a list files operation
type FileListResult struct {
	Files        []FileInfo `json:"files"`
	CurrentPath  string     `json:"current_path"`
}

// FileInfo represents information about a file
type FileInfo struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsDirectory  bool   `json:"is_directory"`
	Size         int64  `json:"size"`
	ModifiedTime int64  `json:"modified_time"`
	CreatedTime  int64  `json:"created_time"`
	Permissions  string `json:"permissions"`
}

// FileReadResult represents the result of a read file operation
type FileReadResult struct {
	Content   []byte `json:"content"`
	TotalSize int64  `json:"total_size"`
}

// FileExistsResult represents the result of a file exists check
type FileExistsResult struct {
	Exists      bool `json:"exists"`
	IsDirectory bool `json:"is_directory"`
}

// DownloadFolderResult represents the result of a download folder operation
type DownloadFolderResult struct {
	Content  []byte `json:"content"`
	Filename string `json:"filename"`
}

// FileCommand represents a file operation command
type FileCommand struct {
	ID         string
	Operation  FileOperation
	Path       string
	SourcePath string
	DestPath   string
	Content    []byte
	Recursive  bool
	IsDir      bool
	Append     bool
	Parents    bool
	Response   chan *FileCommandResult
}

// FileOperation represents the type of file operation
type FileOperation string

const (
	FileOperationList    FileOperation = "list"
	FileOperationCreate  FileOperation = "create"
	FileOperationDelete  FileOperation = "delete"
	FileOperationRename  FileOperation = "rename"
	FileOperationMove    FileOperation = "move"
	FileOperationCopy    FileOperation = "copy"
	FileOperationRead    FileOperation = "read"
	FileOperationWrite   FileOperation = "write"
	FileOperationExists  FileOperation = "exists"
	FileOperationMkdir   FileOperation = "mkdir"
	FileOperationZip     FileOperation = "zip"
	FileOperationUnzip   FileOperation = "unzip"
)

// FileCommandResult represents the result of a file command
type FileCommandResult struct {
	Success bool
	Error   string
	Result  interface{}
}

// ListFiles lists files in a directory on a node
func (m *Manager) ListFiles(ctx context.Context, nodeID string, path string, recursive bool) (*FileListResult, error) {
	cmd := &FileCommand{
		ID:        uuid.New().String(),
		Operation: FileOperationList,
		Path:      path,
		Recursive: recursive,
		Response:  make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf(result.Error)
	}

	if listResult, ok := result.Result.(*FileListResult); ok {
		return listResult, nil
	}

	return nil, fmt.Errorf("invalid result type")
}

// CreateFile creates a file or directory on a node
func (m *Manager) CreateFile(ctx context.Context, nodeID string, path string, isDir bool) error {
	cmd := &FileCommand{
		ID:        uuid.New().String(),
		Operation: FileOperationCreate,
		Path:      path,
		IsDir:     isDir,
		Response:  make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf(result.Error)
	}

	return nil
}

// DeleteFile deletes a file or directory on a node
func (m *Manager) DeleteFile(ctx context.Context, nodeID string, path string, recursive bool) error {
	cmd := &FileCommand{
		ID:        uuid.New().String(),
		Operation: FileOperationDelete,
		Path:      path,
		Recursive: recursive,
		Response:  make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf(result.Error)
	}

	return nil
}

// RenameFile renames a file or directory on a node
func (m *Manager) RenameFile(ctx context.Context, nodeID string, oldPath string, newPath string) error {
	cmd := &FileCommand{
		ID:         uuid.New().String(),
		Operation:  FileOperationRename,
		SourcePath: oldPath,
		DestPath:   newPath,
		Response:   make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf(result.Error)
	}

	return nil
}

// MoveFile moves a file or directory on a node
func (m *Manager) MoveFile(ctx context.Context, nodeID string, sourcePath string, destPath string) error {
	cmd := &FileCommand{
		ID:         uuid.New().String(),
		Operation:  FileOperationMove,
		SourcePath: sourcePath,
		DestPath:   destPath,
		Response:   make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf(result.Error)
	}

	return nil
}

// CopyFile copies a file or directory on a node
func (m *Manager) CopyFile(ctx context.Context, nodeID string, sourcePath string, destPath string, recursive bool) error {
	cmd := &FileCommand{
		ID:         uuid.New().String(),
		Operation:  FileOperationCopy,
		SourcePath: sourcePath,
		DestPath:   destPath,
		Recursive:  recursive,
		Response:   make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf(result.Error)
	}

	return nil
}

// ReadFile reads a file from a node
func (m *Manager) ReadFile(ctx context.Context, nodeID string, path string) (*FileReadResult, error) {
	cmd := &FileCommand{
		ID:        uuid.New().String(),
		Operation: FileOperationRead,
		Path:      path,
		Response:  make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf(result.Error)
	}

	if readResult, ok := result.Result.(*FileReadResult); ok {
		return readResult, nil
	}

	return nil, fmt.Errorf("invalid result type")
}

// WriteFile writes content to a file on a node
func (m *Manager) WriteFile(ctx context.Context, nodeID string, path string, content []byte, append bool) error {
	cmd := &FileCommand{
		ID:        uuid.New().String(),
		Operation: FileOperationWrite,
		Path:      path,
		Content:   content,
		Append:    append,
		Response:  make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf(result.Error)
	}

	return nil
}

// FileExists checks if a file exists on a node
func (m *Manager) FileExists(ctx context.Context, nodeID string, path string) (*FileExistsResult, error) {
	cmd := &FileCommand{
		ID:        uuid.New().String(),
		Operation: FileOperationExists,
		Path:      path,
		Response:  make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf(result.Error)
	}

	if existsResult, ok := result.Result.(*FileExistsResult); ok {
		return existsResult, nil
	}

	return nil, fmt.Errorf("invalid result type")
}

// Mkdir creates a directory on a node
func (m *Manager) Mkdir(ctx context.Context, nodeID string, path string, parents bool) error {
	cmd := &FileCommand{
		ID:        uuid.New().String(),
		Operation: FileOperationMkdir,
		Path:      path,
		Parents:   parents,
		Response:  make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf(result.Error)
	}

	return nil
}

// ZipFiles zips files or folders on a node
func (m *Manager) ZipFiles(ctx context.Context, nodeID string, sourcePath string, destPath string, recursive bool) error {
	cmd := &FileCommand{
		ID:         uuid.New().String(),
		Operation:  FileOperationZip,
		SourcePath: sourcePath,
		DestPath:   destPath,
		Recursive:  recursive,
		Response:   make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf(result.Error)
	}

	return nil
}

// UnzipFiles unzips an archive on a node
func (m *Manager) UnzipFiles(ctx context.Context, nodeID string, sourcePath string, destPath string) error {
	cmd := &FileCommand{
		ID:         uuid.New().String(),
		Operation:  FileOperationUnzip,
		SourcePath: sourcePath,
		DestPath:   destPath,
		Response:   make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf(result.Error)
	}

	return nil
}

// UploadFolder uploads a folder (as zip) to a node
func (m *Manager) UploadFolder(ctx context.Context, nodeID string, destPath string, filename string, content []byte) error {
	// First, write the zip file to a temp location
	zipPath := "/tmp/" + filename
	
	// Write the zip file
	cmd := &FileCommand{
		ID:        uuid.New().String(),
		Operation: FileOperationWrite,
		Path:      zipPath,
		Content:   content,
		Response:  make(chan *FileCommandResult, 1),
	}

	result, err := m.sendFileCommand(ctx, nodeID, cmd)
	if err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf(result.Error)
	}

	// Then unzip it to the destination
	unzipCmd := &FileCommand{
		ID:         uuid.New().String(),
		Operation:  FileOperationUnzip,
		SourcePath: zipPath,
		DestPath:   destPath,
		Response:   make(chan *FileCommandResult, 1),
	}

	unzipResult, err := m.sendFileCommand(ctx, nodeID, unzipCmd)
	if err != nil {
		return err
	}

	if !unzipResult.Success {
		return fmt.Errorf(unzipResult.Error)
	}

	// Clean up the temp zip file
	deleteCmd := &FileCommand{
		ID:        uuid.New().String(),
		Operation: FileOperationDelete,
		Path:      zipPath,
		Response:  make(chan *FileCommandResult, 1),
	}

	m.sendFileCommand(ctx, nodeID, deleteCmd) // Ignore error for cleanup

	return nil
}

// DownloadFolder downloads a folder (as zip) from a node
func (m *Manager) DownloadFolder(ctx context.Context, nodeID string, path string) (*DownloadFolderResult, error) {
	// First, zip the folder
	zipPath := "/tmp/download-" + uuid.New().String() + ".zip"
	
	zipCmd := &FileCommand{
		ID:         uuid.New().String(),
		Operation:  FileOperationZip,
		SourcePath: path,
		DestPath:   zipPath,
		Recursive:  true,
		Response:   make(chan *FileCommandResult, 1),
	}

	zipResult, err := m.sendFileCommand(ctx, nodeID, zipCmd)
	if err != nil {
		return nil, err
	}

	if !zipResult.Success {
		return nil, fmt.Errorf(zipResult.Error)
	}

	// Read the zip file
	readCmd := &FileCommand{
		ID:        uuid.New().String(),
		Operation: FileOperationRead,
		Path:      zipPath,
		Response:  make(chan *FileCommandResult, 1),
	}

	readResult, err := m.sendFileCommand(ctx, nodeID, readCmd)
	if err != nil {
		return nil, err
	}

	if !readResult.Success {
		return nil, fmt.Errorf(readResult.Error)
	}

	// Clean up the temp zip file
	deleteCmd := &FileCommand{
		ID:        uuid.New().String(),
		Operation: FileOperationDelete,
		Path:      zipPath,
		Response:  make(chan *FileCommandResult, 1),
	}

	m.sendFileCommand(ctx, nodeID, deleteCmd) // Ignore error for cleanup

	if readRes, ok := readResult.Result.(*FileReadResult); ok {
		return &DownloadFolderResult{
			Content:  readRes.Content,
			Filename: "folder.zip",
		}, nil
	}

	return nil, fmt.Errorf("invalid result type")
}

// sendFileCommand sends a file command to a node and waits for response
func (m *Manager) sendFileCommand(ctx context.Context, nodeID string, cmd *FileCommand) (*FileCommandResult, error) {
	m.mu.RLock()
	state, exists := m.nodes[nodeID]
	m.mu.RUnlock()

	if !exists {
		// Check if node exists in database but is offline
		node, err := m.nodeRepo.GetByID(ctx, nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to check node existence: %w", err)
		}
		if node == nil {
			return nil, fmt.Errorf("node not found: %s", nodeID)
		}
		// Node exists in DB but is not connected (offline)
		return nil, fmt.Errorf("node is offline: %s (node agent not connected)", nodeID)
	}

	if !state.Connected {
		return nil, fmt.Errorf("node not connected: %s", nodeID)
	}

	// Send command via the node's command queue
	// The command will be processed by the gRPC stream handler
	select {
	case state.CommandQueue <- &Command{
		ID:       cmd.ID,
		Type:     CommandType(cmd.Operation),
		Payload:  cmd,
		Response: make(chan *CommandResult, 1),
	}:
	default:
		return nil, fmt.Errorf("command queue full for node: %s", nodeID)
	}

	// Wait for response with timeout
	select {
	case result := <-cmd.Response:
		return result, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("file command timed out")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
