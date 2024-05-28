//go:build !standalone
// +build !standalone

/*
DESCRIPTION
  youtube.go provides functionality for setting up youtube livestream service
  and broadcast scheduling.

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>
  Dan Kortschak <dan@ausocean.org>
  Russell Stanley <russell@ausocean.org>

LICENSE
  Copyright (C) 2021 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

// Package broadcast provides functionality for setting up a YouTube livestream
// service and broadcast scheduling.
package broadcast

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// Exported status strings.
const (
	StatusComplete = "complete"
	StatusRevoked  = "revoked"
)

// Misc constants.
const (
	// YouTube video category ID for "Science & Technology".
	sciTechCatId = "28"
)

// Exported errors.
var (
	ErrNoBroadcastItems = errors.New("no broadcast items")
)

type IDs struct {
	BID, SID, CID string
}

// getService returns a google authorised and configured youtube service for use
// by the google YouTube API.
func GetService(ctx context.Context, scope string) (*youtube.Service, error) {
	tok, err := getToken(ctx, youtubeCredentials)
	if err != nil {
		return nil, fmt.Errorf("could not get youtube credentials token: %w", err)
	}

	cfg, err := googleConfig(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("could not get google config: %w", err)
	}

	s, err := youtube.NewService(ctx, option.WithHTTPClient(cfg.Client(ctx, tok)))
	if err != nil {
		return nil, fmt.Errorf("could not create youtube service: %w", err)
	}

	if production {
		err = saveTokObj(ctx, tok, youtubeCredentials)
	} else {
		err = saveTokFile(tok, youtubeCredentials)
	}

	if err != nil {
		return nil, fmt.Errorf("could not save new token: %w", err)
	}

	return s, nil
}

// GenerateToken manually generates/regenerates a token. This can be called in
// the case that there's an indication the current token has expired.
func GenerateToken(ctx context.Context, w http.ResponseWriter, r *http.Request, scope string) error {
	cfg, err := googleConfig(ctx, scope)
	if err != nil {
		return fmt.Errorf("could not get google config: %w", err)
	}

	genToken(w, r, cfg, youtubeCredentials)
	return nil
}

// googleConfig creates and returns an oauth2.Config from the provided context
// and scope.
func googleConfig(ctx context.Context, scope string) (*oauth2.Config, error) {
	secrets, err := getSecrets(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get client secrets: %w", err)
	}

	cfg, err := google.ConfigFromJSON(secrets, scope)
	if err != nil {
		return nil, fmt.Errorf("could not create config from client secrets: %w", err)
	}
	return cfg, nil
}

// BroadcastStream uses the youtube API to create and schedule a broadcast with
// a bound stream to which we can send video data.
// This corresponds to the logic in https://github.com/youtube/api-samples/blob/07263305b59a7c3275bc7e925f9ce6cabf774022/python/create_broadcast.py#L135-L138
func BroadcastStream(
	svc *youtube.Service,
	broadcast, description, stream, privacy, resolution, typ, framerate string,
	start, end time.Time,
	log func(string, ...interface{}),
	opts ...googleapi.CallOption) (googleapi.ServerResponse, IDs, error) {

	bID, cID, resp, err := insertBroadcast(svc, broadcast, privacy, start, end, log, opts...)
	ids := IDs{BID: bID, CID: cID}
	if err != nil {
		return resp, ids, fmt.Errorf("could not insert broadcast: %w", err)
	}

	resp, err = setCatAndDesc(svc, broadcast, bID, description, log)
	if err != nil {
		return resp, ids, fmt.Errorf("could not set video category: %w", err)
	}
	log("set category response: %v", resp)

	// stream_id = insert_stream(youtube, args)
	sID, resp, err := insertStream(svc, stream, resolution, typ, framerate, log, opts...)
	if err != nil {
		return resp, ids, fmt.Errorf("could not insert stream: %w", err)
	}
	ids.SID = sID

	resp, err = bindBroadcast(svc, bID, sID, log, opts...)
	if err != nil {
		return resp, ids, fmt.Errorf("could not bind broadcast: %w", err)
	}

	// bind_broadcast(youtube, broadcast_id, stream_id)
	return resp, ids, nil
}

// CompleteBroadcast uses the YouTube API to set the broadcast status of the broadcast with
// bId to "complete".
func CompleteBroadcast(svc *youtube.Service, bID string, log func(string, ...interface{})) error {
	return transition("complete", bID, 0, youtube.NewLiveBroadcastsService(svc), log)
}

// PostChatMessage posts the provided message to the chat with the provided
// chat identification.
func PostChatMessage(cID, msg string) error {
	svc, err := GetService(context.Background(), youtube.YoutubeScope)
	if err != nil {
		return fmt.Errorf("could not get youtube service: %w", err)
	}
	// Build live chat message.
	chatMessage := &youtube.LiveChatMessage{
		Snippet: &youtube.LiveChatMessageSnippet{
			LiveChatId: cID,
			Type:       "textMessageEvent",
			TextMessageDetails: &youtube.LiveChatTextMessageDetails{
				MessageText: msg,
			},
		},
	}
	_, err = youtube.NewLiveChatMessagesService(svc).Insert([]string{"snippet"}, chatMessage).Do()
	if err != nil {
		return fmt.Errorf("could not post live chat message: %v", err)
	}
	return nil
}

// Start transitions a youtube broadcast object into the live state and calls the provided
// extStart function to start external streaming hardware. extStop is called in the case of
// issues and retry. The live broadcast link is provided to the saveLink function and once
// we are live the onLiveActions function is called.
func Start(
	name, bID, sID string,
	saveLink func(key, link string) error,
	extStart, extStop func() error,
	notify func(msg string) error,
	onLiveActions func() error,
	log func(string, ...interface{}),
) error {
	log("starting youtube broadcast object")
	err := doStatusActions(bID, sID, log)
	if err != nil {
		return fmt.Errorf("broadcast: %s, ID: %s, could not do status actions: %w", name, bID, err)
	}

	// If the saveLink function has been provided store the link of the broadcast with the broadcast
	// name (without spaces) as the key.
	if saveLink != nil {
		err := saveLink(strings.ReplaceAll(name, " ", ""), "https://www.youtube.com/watch?v="+bID)
		if err != nil {
			logAndNotify(notify, "broadcast: %s, ID: %s, could not save livestream link: %v", name, bID, err)
		}
	}

	err = onLiveActions()
	if err != nil {
		return fmt.Errorf("broadcast: %s, ID: %s, could not perform on live actions: %v", name, bID, err)
	}

	return nil
}

// logAndNotify is intended for use by background processes when an error must be
// indicated, such as within the goLive routine, which monitors and controls
// a broadcast.
func logAndNotify(notify func(msg string) error, msg string, args ...interface{}) {
	log.Printf(msg, args...)
	err := notify(fmt.Sprintf(msg, args...))
	if err != nil {
		log.Printf("could not send notification: %v", err)
	}
}

// doStatusActions performs a series of status waits and status transitions
// required for live status.
func doStatusActions(bID, sID string, log func(string, ...interface{})) error {
	svc, err := GetService(context.Background(), youtube.YoutubeScope)
	if err != nil {
		return fmt.Errorf("could not get youtube service: %w", err)
	}

	bSvc := youtube.NewLiveBroadcastsService(svc)
	sSvc := youtube.NewLiveStreamsService(svc)

	statusActions := []struct {
		action     func(status, id string, timeout time.Duration, svc interface{}, log func(string, ...interface{})) error
		status, id string
		timeout    time.Duration
		svc        interface{} // Acceptable types are *youtube.LiveBroadcastsService and *youtube.LiveStreamsService.
	}{
		{waitStatus, "ready", bID, 1 * time.Minute, bSvc},
		{waitStatus, "active", sID, 3 * time.Minute, sSvc},
		{robustTransition, "testing", bID, 0, bSvc},
		{waitStatus, "testing", bID, 1 * time.Minute, bSvc},
		{transition, "live", bID, 0, bSvc},
		{waitStatus, "live", bID, 1 * time.Minute, bSvc},
	}

	for i, v := range statusActions {
		err := v.action(v.status, v.id, v.timeout, v.svc, log)
		if err != nil {
			return fmt.Errorf("failed to go live, could not perform status action: %d: %w", i, err)
		}
	}
	return nil
}

// waitStatus waits for the given status on the broadcast or stream with given
// id. The wait will terminate if timeout is exceeded.
// Accepted types for svc are *youtube.LiveBroadcastsService and
// *youtube.LiveStreamsService.
func waitStatus(status, id string, timeout time.Duration, svc interface{}, log func(string, ...interface{})) error {
	const checkIntvl = 15 * time.Second
	chk := time.NewTicker(checkIntvl)
	tmo := time.NewTimer(timeout)

	log("waiting for %s status...", status)
	for {
		select {
		case <-tmo.C: // Timeout..
			return fmt.Errorf("status wait timeout exceeded for %s status", status)
		case <-chk.C: // Ticker (check status).
			var s string
			var err error
			switch svc := svc.(type) {
			case *youtube.LiveBroadcastsService:
				s, err = getBroadcastStatus(svc, id)
			case *youtube.LiveStreamsService:
				s, err = getStreamStatus(svc, id)
			default:
				panic("unexpected service type")
			}
			if err != nil {
				return fmt.Errorf("could not get status: %w", err)
			}

			if s == status {
				log("status %s reached, breaking...", status)
				return nil
			}
		}
	}
}

// GetBroadcastStatus gets the status of the broadcast with the provided ID.
func GetBroadcastStatus(svc *youtube.Service, id string) (string, error) {
	return getBroadcastStatus(youtube.NewLiveBroadcastsService(svc), id)
}

// GetBroadcastStatus gets the status of the broadcast with the given id.
func getBroadcastStatus(svc *youtube.LiveBroadcastsService, id string) (string, error) {
	resp, err := svc.List([]string{"status"}).Id(id).Do()
	if err != nil {
		return "", fmt.Errorf("could not list broadcasts: %w", err)
	}
	if len(resp.Items) == 0 {
		return "", ErrNoBroadcastItems
	}
	return resp.Items[0].Status.LifeCycleStatus, nil
}

// getStreamStatuses retrieves the LiveStreamStatus struct for the given ID.
func getStreamStatuses(svc *youtube.LiveStreamsService, id string) (*youtube.LiveStreamStatus, error) {
	resp, err := svc.List([]string{"status"}).Id(id).Do()
	if err != nil {
		return nil, fmt.Errorf("could not list streams: %w", err)
	}
	if len(resp.Items) == 0 {
		return nil, errors.New("no stream items")
	}
	return resp.Items[0].Status, nil
}

// getStreamStatus provides the string stream status for the stream of given ID.
func getStreamStatus(svc *youtube.LiveStreamsService, id string) (string, error) {
	statuses, err := getStreamStatuses(svc, id)
	if err != nil {
		return "", fmt.Errorf("could not get statuses")
	}
	return statuses.StreamStatus, nil
}

// getHealthStatus provides the LiveStreamHealthStatus struct for the given
// stream ID.
func GetHealthStatus(svc *youtube.Service, id string) (*youtube.LiveStreamHealthStatus, error) {
	statuses, err := getStreamStatuses(youtube.NewLiveStreamsService(svc), id)
	if err != nil {
		return nil, fmt.Errorf("could not get statuses: %w", err)
	}
	return statuses.HealthStatus, nil
}

// robustTransition transitions to another status with more leniency on errors
// that might be caused by temporary inactive periods i.e. we retry up to
// transitionMaxTries before returning with error.
func robustTransition(status, id string, timeout time.Duration, svc interface{}, log func(string, ...interface{})) error {
	const (
		transitionMaxTries = 3
		retryWait          = 5 * time.Second
	)
	var err error
	for i := 0; i < transitionMaxTries; i++ {
		err = transition(status, id, timeout, svc, log)
		if err != nil {
			log("transition to %s for %s failed on attempt %d with error: %v", status, id, i, err)
			time.Sleep(retryWait)
			continue
		}
		return nil
	}
	return fmt.Errorf("could not transition to %s, max tries exceeded: %w", status, err)
}

// transition transitions to the given status for broadcast with the given id.
// svc must have underlying type of *youtube.LiveBroadcastsService.
func transition(status, id string, timeout time.Duration, svc interface{}, log func(string, ...interface{})) error {
	log("ID: %s, requesting transition to %s status...", id, status)
	_, err := svc.(*youtube.LiveBroadcastsService).Transition(status, id, []string{"status"}).Do()
	return err
}

// insertBroadcast corresponds to https://github.com/youtube/api-samples/blob/07263305b59a7c3275bc7e925f9ce6cabf774022/python/create_broadcast.py#L63-L84
func insertBroadcast(svc *youtube.Service, broadcast, privacy string, start, end time.Time, log func(string, ...interface{}), opts ...googleapi.CallOption) (id, chatId string, servResp googleapi.ServerResponse, err error) {
	log("inserting broadcast, name: %s, privacy: %s, start: %v, end: %v", broadcast, privacy, start, end)
	// broadcast_id = insert_broadcast(youtube, args)
	b := youtube.NewLiveBroadcastsService(svc)
	resp, err := b.Insert([]string{"snippet", "status"}, &youtube.LiveBroadcast{
		Snippet: &youtube.LiveBroadcastSnippet{
			Title:              broadcast,
			ScheduledStartTime: start.Format(time.RFC3339),
			ScheduledEndTime:   end.Format(time.RFC3339),
		},
		Status: &youtube.LiveBroadcastStatus{
			PrivacyStatus:           privacy,
			SelfDeclaredMadeForKids: false,
			ForceSendFields:         []string{"SelfDeclaredMadeForKids"},
		},
	}).Do(opts...)
	if err != nil {
		if resp != nil {
			return "", "", resp.ServerResponse, err
		}
		return "", "", googleapi.ServerResponse{}, err
	}
	log("Broadcast %q with title %q was published at %v.",
		resp.Id, resp.Snippet.Title, resp.Snippet.PublishedAt)

	return resp.Id, resp.Snippet.LiveChatId, resp.ServerResponse, nil
}

// setCatAndDesc sets the category and description for the broadcast. The category
// is set to "Science & Technology".
func setCatAndDesc(svc *youtube.Service, title, id, description string, log func(string, ...interface{})) (googleapi.ServerResponse, error) {
	log("setting category to \"Science & Technology\" for ID: %s", id)
	v := youtube.NewVideosService(svc)
	resp, err := v.Update([]string{"snippet"}, &youtube.Video{
		Id: id,
		Snippet: &youtube.VideoSnippet{
			CategoryId:  sciTechCatId,
			Title:       title,
			Description: description,
		},
	}).Do()
	if err != nil {
		if resp != nil {
			return resp.ServerResponse, err
		}
		return googleapi.ServerResponse{}, err
	}
	return resp.ServerResponse, nil
}

// insertStream corresponds to https://github.com/youtube/api-samples/blob/07263305b59a7c3275bc7e925f9ce6cabf774022/python/create_broadcast.py#L86-L106
func insertStream(svc *youtube.Service, stream, resolution, typ, framerate string, log func(string, ...interface{}), opts ...googleapi.CallOption) (id string, servResp googleapi.ServerResponse, err error) {
	log("inserting stream, name: %s, res: %s, typ: %s, rate: %s", stream, resolution, typ, framerate)
	s := youtube.NewLiveStreamsService(svc)
	resp, err := s.Insert([]string{"snippet", "cdn"}, &youtube.LiveStream{
		Snippet: &youtube.LiveStreamSnippet{
			Title: stream,
		},
		Cdn: &youtube.CdnSettings{
			Resolution:    resolution,
			IngestionType: typ,
			FrameRate:     framerate,
		},
	}).Do(opts...)
	if err != nil {
		if resp != nil {
			return "", resp.ServerResponse, err
		}
		return "", googleapi.ServerResponse{}, err
	}
	log("Stream %q with title %q was inserted.",
		resp.Id, resp.Snippet.Title)
	return resp.Id, resp.ServerResponse, nil
}

// bindBroadcast corresponds to https://github.com/youtube/api-samples/blob/07263305b59a7c3275bc7e925f9ce6cabf774022/python/create_broadcast.py#L108-L119
func bindBroadcast(svc *youtube.Service, bID, sID string, log func(string, ...interface{}), opts ...googleapi.CallOption) (googleapi.ServerResponse, error) {
	resp, err := svc.LiveBroadcasts.Bind(bID, []string{"id", "contentDetails"}).StreamId(sID).Do(opts...)
	if err != nil {
		return resp.ServerResponse, err
	}
	log("Broadcast %q was bound to stream %q.",
		resp.Id, resp.ContentDetails.BoundStreamId)
	return resp.ServerResponse, nil
}

// RTMPKey retrieves the RTMP Key required for appending to the RTMP destination
// URL given to the encoder, for the provided stream title.
func RTMPKey(svc *youtube.Service, title string) (string, error) {
	resp, err := youtube.NewLiveStreamsService(svc).List([]string{"snippet", "cdn", "status"}).Mine(true).Do()
	if err != nil {
		return "", fmt.Errorf("could not perform livestreams listing: %w", err)
	}
	for _, item := range resp.Items {
		if item.Snippet.Title == title {
			return item.Cdn.IngestionInfo.StreamName, nil
		}
	}
	return "", fmt.Errorf("could not find stream with title: %s", title)
}
