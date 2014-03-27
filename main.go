package main

import (
	"errors"
	"github.com/gedex/go-instagram/instagram"
	"log"
	"math"
	"os"
	"strings"
	"time"
)

var (
	EnvAccessToken          = "ACCESS_TOKEN"
	EnvHashtags             = "HASHTAGS"
	CommentInterval         = time.Duration(5) * time.Minute
	RecentMediaPollInterval = time.Duration(20) * time.Second
	HashtagBatchSize        = 4
	hashtags                []string
)

var client *instagram.Client

func apiGetRecentMedia(sinceDate time.Time) ([]instagram.Media, error) {
	media, _, err := client.Users.RecentMedia("self", &instagram.Parameters{MinTimestamp: sinceDate.Unix() - 1})
	return media, err
}

func apiDeleteComment(mediaId, commentId string) error {
	return client.Comments.Delete(mediaId, commentId)
}

func apiAddComment(mediaId string, text string) (string, error) {
	err := client.Comments.Add(mediaId, []string{text})
	if err != nil {
		return "", err
	}
	log.Printf("TRACE: Comment added on media %s", mediaId)

	// Give 3 seconds of delay since I have seen API not immediately returning the submitted comment
	<-time.NewTimer(time.Duration(3) * time.Second).C

	// Find id of the comment among all comments by comparing with exact full text
	comments, err := client.Comments.MediaComments(mediaId)
	if err != nil {
		return "", err
	}
	log.Printf("TRACE: Found %d comments on media %s", len(comments), mediaId)

	var id string
	for _, c := range comments {
		if c.Text == text {
			id = c.ID
			break
		}
	}

	if id == "" {
		log.Printf("TRACE: Can't find comment just posted in comments response. Trying caption of the picture.")
	}
	m, err := client.Media.Get(mediaId)
	if err != nil {
		return "", err
	}
	if m.Caption.Text == text {
		log.Printf("TRACE: Found the comment posted as caption on media %s.", mediaId)
		id = m.Caption.ID
	}

	if id == "" {
		return "", errors.New("instagger: Cannot find comment I just added. Possibly will leak that.")
	} else {
		log.Printf("TRACE: Comment added on media %s has found with id %s.", mediaId, id)
	}
	return id, nil
}

func process(m instagram.Media, tagBatches [][]string) {
	for i, batch := range tagBatches {
		commentText := strings.Join(batch, " ")
		commentId, err := apiAddComment(m.ID, commentText)
		if err != nil {
			log.Printf("ERROR: while adding comment on %s: %s", m.ID, err)
		}

		if commentId == "" {
			log.Printf("WARN: Received empty id from apiComment function on media %s, possibly a leak.", m.ID)
			break
		}

		// Sleep for a while
		timer := time.NewTimer(CommentInterval)
		<-timer.C

		// Delete the comment
		err = apiDeleteComment(m.ID, commentId)
		if err != nil {
			log.Printf("ERROR: while deleting comment %s on %s: %s", commentId, m.ID, err)
			break
		}
		log.Printf("TRACE: Comment %s deleted on media %s.", commentId, m.ID)

		log.Printf("TRACE: Batch #%d completed for media %s.", i+1, m.ID)
	}

	log.Printf("TRACE: Done processing media %s.", m.ID)

	// Refresh media object to read final number of likes.
	media, err := client.Media.Get(m.ID)
	if err == nil {
		log.Printf("TRACE: Total likes for %s is >>> %d <<<", media.ID, media.Likes.Count)
	}
}

func getHashtagBatches() [][]string {
	arr := make([][]string, int(math.Ceil(float64(len(hashtags))/float64(HashtagBatchSize))))
	for i, h := range hashtags {
		n := i / HashtagBatchSize
		arr[n] = append(arr[n], h)
	}
	return arr
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(os.Stdout)
	client = instagram.NewClient(nil)

	if os.Getenv(EnvAccessToken) == "" {
		log.Fatalf("FATAL: Can't find environment variable %s.", EnvAccessToken)
	}
	client.AccessToken = os.Getenv(EnvAccessToken)

	if os.Getenv(EnvHashtags) == "" {
		log.Fatalf("FATAL: Can't find environment variable %s. Provide a comma-separated list.", EnvHashtags)
	}
	hashtags = strings.Split(os.Getenv(EnvHashtags), ",")

	tagBatches := getHashtagBatches()
	log.Printf("INFO: Found %d hashtags, configured batch size: %d", len(hashtags), HashtagBatchSize)
	log.Printf("INFO: Hashtag batches configured: %s", tagBatches)
	log.Printf("INFO: Total hashtag batch count is: %d", len(tagBatches))
	log.Printf("INFO: End-to-end completion time for a media: ~%s", CommentInterval*time.Duration(len(tagBatches)))
	log.Printf("INFO: Configured CommentInterval: %s", CommentInterval)
	log.Printf("INFO: Configured RecentMediaPollInterval: %s", RecentMediaPollInterval)
	log.Printf("TRACE: Starting main loop.\n")

	minDate := time.Now()
	ticker := time.NewTicker(RecentMediaPollInterval)
	for t := range ticker.C {
		media, err := apiGetRecentMedia(minDate)
		if err != nil {
			log.Printf("ERROR: instagram: error while retrieving recent media: %s", err)
		}

		if len(media) > 0 {
			log.Printf("INFO: Tick... New media found: %d since %d", len(media), minDate.Unix())
		} else {
			log.Printf("TRACE: Tick... No new media found since %d", minDate.Unix())
		}

		minDate = t

		for _, m := range media {
			go process(m, tagBatches)
		}
	}
	log.Panicf("ERROR: main loop should have never exited.")
}
