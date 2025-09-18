/*
DESCRIPTION
  upload.go provides functionality for uploading videos to YouTube

AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2025 the Australian Ocean Lab (AusOcean)

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

package youtube

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/utils"
	"google.golang.org/api/youtube/v3"
)

var ErrUnknownStatus = errors.New("unknown video status")

// VideoUploadOption is a functional option type for configuring YouTube video uploads.
type VideoUploadOption func(*youtube.Video) error

// WithTitle sets the title of the video being uploaded.
// It returns an error if the title is empty.
func WithTitle(title string) VideoUploadOption {
	return func(video *youtube.Video) error {
		if title == "" {
			return fmt.Errorf("title cannot be empty")
		}
		video.Snippet.Title = title
		return nil
	}
}

// WithDescription sets the description of the video being uploaded.
// It returns an error if the description is empty.
func WithDescription(description string) VideoUploadOption {
	return func(video *youtube.Video) error {
		if description == "" {
			return fmt.Errorf("description cannot be empty")
		}
		video.Snippet.Description = description
		return nil
	}
}

// WithCategory sets the category of the video being uploaded.
// It accepts either a category ID or a category name (both as strings) from the following:
//
// 1 - Film & Animation
// 2 - Autos & Vehicles
// 10 - Music
// 15 - Pets & Animals
// 17 - Sports
// 18 - Short Movies
// 19 - Travel & Events
// 20 - Gaming
// 21 - Videoblogging
// 22 - People & Blogs
// 23 - Comedy
// 24 - Entertainment
// 25 - News & Politics
// 26 - Howto & Style
// 27 - Education
// 28 - Science & Technology
// 29 - Nonprofits & Activism
// 30 - Movies
// 31 - Anime/Animation
// 32 - Action/Adventure
// 33 - Classics
// 34 - Comedy
// 35 - Documentary
// 36 - Drama
// 37 - Family
// 38 - Foreign
// 39 - Horror
// 40 - Sci-Fi/Fantasy
// 41 - Thriller
// 42 - Shorts
// 43 - Shows
// 44 - Trailers
//
// If a name is provided, it will be matched against a predefined list of categories
// and the corresponding ID will be used.
// It returns an error if the category ID/name is not found.
func WithCategory(categoryID string) VideoUploadOption {
	return func(video *youtube.Video) error {
		video.Snippet.CategoryId = sanitiseCategory(categoryID)
		if video.Snippet.CategoryId == "" {
			return fmt.Errorf("invalid category ID or name: %s", categoryID)
		}
		return nil
	}
}

// WithPrivacy sets the privacy status of the video being uploaded.
// It accepts "public", "unlisted", or "private" as valid privacy statuses.
// It returns an error if the privacy status is empty or invalid.
func WithPrivacy(privacy string) VideoUploadOption {
	return func(video *youtube.Video) error {
		if !validPrivacy(privacy) {
			return fmt.Errorf("invalid privacy status: %s", privacy)
		}
		video.Status.PrivacyStatus = privacy
		return nil
	}
}

// WithTags sets the tags for the video being uploaded.
// It returns an error if the tags slice is empty.
func WithTags(tags []string) VideoUploadOption {
	return func(video *youtube.Video) error {
		if len(tags) == 0 {
			return fmt.Errorf("tags cannot be empty")
		}
		video.Snippet.Tags = tags
		return nil
	}
}

// Upload Status constants.
const (
	UploadStatusUploaded  = "uploaded"
	UploadStatusProcessed = "processed"
	UploadStatusFailed    = "failed"
	UploadStatusRejected  = "rejected"
	UploadStatusDeleted   = "deleted"
)

// UploadVideo uploads a video to AusOcean's YouTube account using the provided media reader and options.
// Defaults are applied for title, description, category, privacy, and tags if not specified in options.
// Defaults are as follows:
// - Title: "Uploaded at <current time>"
// - Description: "No description provided."
// - Category: "Science & Technology" (ID: 28)
// - Privacy: "unlisted"
// - Tags: ["ocean uploads"]
// It returns an error if the upload fails.
func UploadVideo(ctx context.Context, media io.Reader, opts ...VideoUploadOption) (*youtube.Video, error) {
	const (
		// Science & Technology category ID
		scienceAndTechnologyCategoryID = "28"

		// Defaults
		defaultDescription = "No description provided."
		defaultCategory    = scienceAndTechnologyCategoryID
		defaultPrivacy     = "unlisted"
	)

	// Defaults
	var (
		defaultTitle    = "Uploaded at " + time.Now().Format("2006-01-02 15:04:05")
		defaultKeywords = []string{"ocean uploads"}
	)

	upload := &youtube.Video{
		Snippet: &youtube.VideoSnippet{
			Title:       defaultTitle,
			Description: defaultDescription,
			CategoryId:  defaultCategory,
			Tags:        defaultKeywords, // The API returns a 400 Bad Request response if tags is an empty string.
		},
		Status: &youtube.VideoStatus{PrivacyStatus: defaultPrivacy},
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(upload); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Force using the default account (AusOcean's account).
	tokenURI := utils.TokenURIFromAccount("")
	svc, err := broadcast.GetService(ctx, youtube.YoutubeScope, tokenURI)
	if err != nil {
		return nil, fmt.Errorf("failed to get YouTube service: %w", err)
	}

	vid, err := youtube.NewVideosService(svc).Insert([]string{"snippet", "status"}, upload).Media(media).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to insert video: %w", err)
	}

	return vid, nil
}

// CheckUploadStatus checks the status for the video with the associated videoID.
// the returned status will be one of:
// - UploadStatusUploaded
// - UploadStatusProcessed
// - UploadStatusFailed
// - UploadStatusRejected
// - UploadStatusDeleted
func CheckUploadStatus(ctx context.Context, videoID string) (string, error) {
	// Force using the default account (AusOcean's account).
	tokenURI := utils.TokenURIFromAccount("")
	svc, err := broadcast.GetService(ctx, youtube.YoutubeScope, tokenURI)
	if err != nil {
		return "", fmt.Errorf("failed to get YouTube service: %w", err)
	}
	vid, err := youtube.NewVideosService(svc).List([]string{"snippet", "status"}).Id(videoID).Do()
	if err != nil {
		return "", fmt.Errorf("failed to get video status: %w", err)
	}

	if len(vid.Items) == 0 {
		return "", fmt.Errorf("video not found")
	}

	switch vid.Items[0].Status.UploadStatus {
	case "processed":
		return UploadStatusProcessed, nil
	case "failed":
		return UploadStatusFailed, nil
	case "rejected":
		return UploadStatusRejected, nil
	case "deleted":
		return UploadStatusDeleted, nil
	case "uploaded":
		return UploadStatusUploaded, nil
	default:
		return "", ErrUnknownStatus
	}
}

// sanitiseCategory checks if the given category ID or Name is valid,
// and returns its ID if valid.
func sanitiseCategory(cat string) string {
	categories := map[string]string{
		"1":  "Film & Animation",
		"2":  "Autos & Vehicles",
		"10": "Music",
		"15": "Pets & Animals",
		"17": "Sports",
		"18": "Short Movies",
		"19": "Travel & Events",
		"20": "Gaming",
		"21": "Videoblogging",
		"22": "People & Blogs",
		"23": "Comedy",
		"24": "Entertainment",
		"25": "News & Politics",
		"26": "Howto & Style",
		"27": "Education",
		"28": "Science & Technology",
		"29": "Nonprofits & Activism",
		"30": "Movies",
		"31": "Anime/Animation",
		"32": "Action/Adventure",
		"33": "Classics",
		"34": "Comedy",
		"35": "Documentary",
		"36": "Drama",
		"37": "Family",
		"38": "Foreign",
		"39": "Horror",
		"40": "Sci-Fi/Fantasy",
		"41": "Thriller",
		"42": "Shorts",
		"43": "Shows",
		"44": "Trailers",
	}
	for id, name := range categories {
		if id == cat || name == cat {
			return id
		}
	}
	return ""
}

func validPrivacy(privacy string) bool {
	validStatuses := map[string]bool{
		"public":   true,
		"unlisted": true,
		"private":  true,
	}
	return validStatuses[privacy]
}
