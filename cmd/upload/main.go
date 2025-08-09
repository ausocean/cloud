/*
DESCRIPTION
  upload provides a simple command-line utility for uploading videos to
  AusOcean's YouTube account.

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

// Upload provides a command-line utility for uploading videos to
// AusOcean's YouTube account.
// It uses the YouTube Data API v3 to handle video uploads and metadata.
// It assumes YouTube secrets are pointed at by an environment variable
// YOUTUBE_SECRETS, which should contain the path to a JSON file.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/ausocean/cloud/youtube"
)

func main() {
	media := flag.String("media", "", "Path to the video file to upload")
	flag.Parse()

	// Create io.Reader for the media file
	reader, err := os.Open(*media)
	if err != nil {
		log.Fatalf("Failed to create media reader: %v", err)
	}

	// Example usage
	err = youtube.UploadVideo(
		context.Background(),
		reader,
		youtube.WithTitle("Test upload "+time.Now().Format("2006-01-02 15:04:05")),
		youtube.WithDescription("This is a test upload"),
		youtube.WithCategory("28"), // Science & Technology
		youtube.WithPrivacy("unlisted"),
		youtube.WithTags([]string{"test", "upload"}),
	)
	if err != nil {
		log.Fatalf("Failed to upload video: %v", err)
	}
}
