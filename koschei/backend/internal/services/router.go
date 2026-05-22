package services

import "koschei/backend/internal/config"

type TaskType string

const (
	TaskChat      TaskType = "chat"
	TaskCode      TaskType = "code"
	TaskReason    TaskType = "reason"
	TaskImage     TaskType = "image"
	TaskImageEdit TaskType = "image_edit"
	TaskVideo     TaskType = "video"
	TaskCinema    TaskType = "cinema"
	TaskTTS       TaskType = "tts"
	TaskSTT       TaskType = "stt"
)

type AIRouter struct {
	Cfg config.Config
}

func (r AIRouter) SelectModel(task TaskType) string {
	switch task {
	case TaskCode:
		return r.Cfg.TogetherModelCode
	case TaskReason:
		return r.Cfg.TogetherModelReasoning
	case TaskImage:
		return r.Cfg.TogetherModelImage
	case TaskImageEdit:
		return r.Cfg.TogetherModelImageEdit
	case TaskVideo:
		return r.Cfg.TogetherModelVideo
	case TaskCinema:
		return r.Cfg.TogetherModelVideoCine
	case TaskTTS:
		return r.Cfg.TogetherModelTTS
	case TaskSTT:
		return r.Cfg.TogetherModelSTT
	default:
		return r.Cfg.TogetherModelChat
	}
}
