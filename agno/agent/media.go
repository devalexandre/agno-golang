package agent

import "github.com/devalexandre/agno-golang/agno/models"

// Media supplied to a run is attached to the last user message so it reaches the
// model. Provider clients convert it to their own wire format and skip whatever
// they cannot represent (see models/media.go).
//
// The run options carry agent-level media types; these convert them to the
// canonical types the model layer understands.

func (i Image) toModel() models.Image {
	return models.Image{ID: i.ID, URL: i.URL, Data: i.Data, MimeType: i.MimeType}
}

func (a Audio) toModel() models.Audio {
	return models.Audio{ID: a.ID, URL: a.URL, Data: a.Data, MimeType: a.MimeType}
}

func (v Video) toModel() models.Video {
	return models.Video{ID: v.ID, URL: v.URL, Data: v.Data, MimeType: v.MimeType}
}

func (f File) toModel() models.File {
	return models.File{ID: f.ID, Name: f.Name, URL: f.URL, Data: f.Data, MimeType: f.MimeType}
}

// attachRunMedia attaches the run's media to the last user message in messages.
// It is a no-op when the run carries no media, or when there is no user message
// to attach it to.
func attachRunMedia(messages []models.Message, images []Image, audio []Audio, videos []Video, files []File) []models.Message {
	if len(images) == 0 && len(audio) == 0 && len(videos) == 0 && len(files) == 0 {
		return messages
	}

	idx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == models.TypeUserRole {
			idx = i
			break
		}
	}
	if idx < 0 {
		return messages
	}

	for _, img := range images {
		messages[idx].Images = append(messages[idx].Images, img.toModel())
	}
	for _, aud := range audio {
		messages[idx].Audio = append(messages[idx].Audio, aud.toModel())
	}
	for _, vid := range videos {
		messages[idx].Videos = append(messages[idx].Videos, vid.toModel())
	}
	for _, f := range files {
		messages[idx].Files = append(messages[idx].Files, f.toModel())
	}

	return messages
}
