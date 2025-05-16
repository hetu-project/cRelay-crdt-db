package orbitdb

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"berty.tech/go-orbit-db/iface"
	"github.com/nbd-wtf/go-nostr"
)

// UserStats represents user statistics data
type UserStats struct {
	ID               string                       `json:"id"`                     // User ID, which is the user's ETH address
	DocType          string                       `json:"doc_type"`               // Document type, fixed as "user_stats"
	TotalStats       map[uint32]uint64            `json:"total_stats"`            // Overall statistics for various operations
	SubspaceStats    map[string]map[uint32]uint64 `json:"subspace_stats"`         // Statistics for each subspace
	CreatedSubspaces []string                     `json:"created_subspaces"`      // List of subspace IDs created by the user
	JoinedSubspaces  []string                     `json:"joined_subspaces"`       // List of subspace IDs joined by the user
	VoteStats        *VoteStats                   `json:"vote_stats,omitempty"`   // Voting statistics
	InviteStats      *InviteStats                 `json:"invite_stats,omitempty"` // Invitation statistics
	LastUpdated      int64                        `json:"last_updated"`           // Last update time
}

// VoteStats represents voting-related statistics
type VoteStats struct {
	TotalVotes    uint64                        `json:"total_votes"`    // Total number of votes
	YesVotes      uint64                        `json:"yes_votes"`      // Total number of yes votes
	NoVotes       uint64                        `json:"no_votes"`       // Total number of no votes
	SubspaceVotes map[string]*SubspaceVoteStats `json:"subspace_votes"` // Voting statistics for each subspace
}

// SubspaceVoteStats represents voting statistics for a subspace
type SubspaceVoteStats struct {
	TotalVotes uint64 `json:"total_votes"` // Total number of votes in the subspace
	YesVotes   uint64 `json:"yes_votes"`   // Number of yes votes in the subspace
	NoVotes    uint64 `json:"no_votes"`    // Number of no votes in the subspace
}

// InviteStats represents invitation-related statistics
type InviteStats struct {
	TotalInvited    uint64                        `json:"total_invited"`    // Total number of successful invitations
	SubspaceInvited map[string]uint64             `json:"subspace_invited"` // Number of successful invitations for each subspace
	InvitedUsers    map[string][]*InvitedUserInfo `json:"invited_users"`    // Information about users who accepted invitations
}

// InvitedUserInfo represents information about an invited user
type InvitedUserInfo struct {
	UserID     string `json:"user_id"`     // Invited user's address
	SubspaceID string `json:"subspace_id"` // Subspace the user was invited to join
	Timestamp  int64  `json:"timestamp"`   // Time the invitation was accepted
}

// UserStatsManager manages user statistics
type UserStatsManager struct {
	db iface.DocumentStore
}

// NewUserStatsManager creates a new UserStatsManager
func NewUserStatsManager(db iface.DocumentStore) *UserStatsManager {
	return &UserStatsManager{db: db}
}

// GetUserStats retrieves user statistics
func (um *UserStatsManager) GetUserStats(ctx context.Context, userID string) (*UserStats, error) {
	// Query user data
	docs, err := um.db.Get(ctx, userID, nil)
	if err != nil {
		return nil, err
	}

	// If not found, return nil
	if len(docs) == 0 {
		return nil, nil
	}

	// Iterate through results to find user_stats type document
	var userStatsDoc map[string]interface{}
	for _, doc := range docs {
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			continue
		}

		docType, ok := docMap["doc_type"].(string)
		if !ok || docType != "user_stats" {
			continue
		}

		userStatsDoc = docMap
		break
	}

	if userStatsDoc == nil {
		return nil, nil
	}

	// Convert document to JSON and parse into struct
	jsonData, err := json.Marshal(userStatsDoc)
	if err != nil {
		return nil, err
	}

	var userStats UserStats
	if err := json.Unmarshal(jsonData, &userStats); err != nil {
		return nil, err
	}

	return &userStats, nil
}

// UpdateUserStatsFromEvent updates user statistics from an event
func (um *UserStatsManager) UpdateUserStatsFromEvent(ctx context.Context, event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Get existing user statistics
	userID := event.PubKey
	stats, err := um.GetUserStats(ctx, userID)
	if err != nil {
		return err
	}

	// If not found, create new statistics
	now := time.Now().Unix()
	if stats == nil {
		stats = &UserStats{
			ID:               userID,
			DocType:          "user_stats",
			TotalStats:       make(map[uint32]uint64),
			SubspaceStats:    make(map[string]map[uint32]uint64),
			CreatedSubspaces: []string{},
			JoinedSubspaces:  []string{},
			LastUpdated:      now,
		}
	}

	// Update last update time
	stats.LastUpdated = now

	// Update statistics based on event type
	kind := uint32(event.Kind)

	// Increment total statistics
	stats.TotalStats[kind] = stats.TotalStats[kind] + 1

	// Find subspace ID in event
	var subspaceID string
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "sid" {
			subspaceID = tag[1]
			break
		}
	}

	// If subspace ID exists, update subspace-related statistics
	if subspaceID != "" {
		// Ensure subspace statistics exist
		if _, exists := stats.SubspaceStats[subspaceID]; !exists {
			stats.SubspaceStats[subspaceID] = make(map[uint32]uint64)
		}
		// Increment subspace statistics
		stats.SubspaceStats[subspaceID][kind] = stats.SubspaceStats[subspaceID][kind] + 1

		switch kind {
		case 30100: // Create subspace
			// Add subspace to created subspaces list
			if !containsString(stats.CreatedSubspaces, subspaceID) {
				stats.CreatedSubspaces = append(stats.CreatedSubspaces, subspaceID)
			}

		case 30200: // Join subspace
			// Add subspace to joined subspaces list
			if !containsString(stats.JoinedSubspaces, subspaceID) {
				stats.JoinedSubspaces = append(stats.JoinedSubspaces, subspaceID)
			}

		case 30302: // Vote
			// Initialize vote statistics
			if stats.VoteStats == nil {
				stats.VoteStats = &VoteStats{
					SubspaceVotes: make(map[string]*SubspaceVoteStats),
				}
			}

			// Ensure subspace vote statistics exist
			if _, exists := stats.VoteStats.SubspaceVotes[subspaceID]; !exists {
				stats.VoteStats.SubspaceVotes[subspaceID] = &SubspaceVoteStats{}
			}

			// Increment total vote count
			stats.VoteStats.TotalVotes++
			// Increment subspace vote count
			stats.VoteStats.SubspaceVotes[subspaceID].TotalVotes++

			// Check vote type (yes/no)
			for _, tag := range event.Tags {
				if len(tag) >= 2 && tag[0] == "vote" {
					voteValue := tag[1]
					if voteValue == "yes" {
						stats.VoteStats.YesVotes++
						stats.VoteStats.SubspaceVotes[subspaceID].YesVotes++
					} else if voteValue == "no" {
						stats.VoteStats.NoVotes++
						stats.VoteStats.SubspaceVotes[subspaceID].NoVotes++
					}
					break
				}
			}

		case 30303: // Invite
			// Handle invitation acceptance
			var inviterAddr string
			for _, tag := range event.Tags {
				if len(tag) >= 2 && tag[0] == "inviter_addr" {
					inviterAddr = tag[1]
					break
				}
			}

			if inviterAddr != "" {
				// The inviter is another user, the current user is the invitee
				// Need to update inviter's statistics
				err = um.updateInviterStats(ctx, inviterAddr, userID, subspaceID, now)
				if err != nil {
					log.Printf("Failed to update inviter statistics: %v", err)
				}
			}
		}
	}

	// Save updated statistics
	return um.saveUserStats(ctx, stats)
}

// Update inviter's invitation statistics
func (um *UserStatsManager) updateInviterStats(ctx context.Context, inviterID, invitedID, subspaceID string, timestamp int64) error {
	// Get inviter's statistics
	inviterStats, err := um.GetUserStats(ctx, inviterID)
	if err != nil {
		return err
	}

	// If not found, create new statistics
	if inviterStats == nil {
		inviterStats = &UserStats{
			ID:            inviterID,
			DocType:       "user_stats",
			TotalStats:    make(map[uint32]uint64),
			SubspaceStats: make(map[string]map[uint32]uint64),
			LastUpdated:   timestamp,
		}
	}

	// Initialize invitation statistics
	if inviterStats.InviteStats == nil {
		inviterStats.InviteStats = &InviteStats{
			SubspaceInvited: make(map[string]uint64),
			InvitedUsers:    make(map[string][]*InvitedUserInfo),
		}
	}

	// Update invitation statistics
	inviterStats.InviteStats.TotalInvited++
	inviterStats.InviteStats.SubspaceInvited[subspaceID]++

	// Add invited user information
	userInfo := &InvitedUserInfo{
		UserID:     invitedID,
		SubspaceID: subspaceID,
		Timestamp:  timestamp,
	}

	// Append to user list
	if _, exists := inviterStats.InviteStats.InvitedUsers[subspaceID]; !exists {
		inviterStats.InviteStats.InvitedUsers[subspaceID] = []*InvitedUserInfo{}
	}
	inviterStats.InviteStats.InvitedUsers[subspaceID] = append(inviterStats.InviteStats.InvitedUsers[subspaceID], userInfo)

	// Save updated statistics
	return um.saveUserStats(ctx, inviterStats)
}

// Save user statistics
func (um *UserStatsManager) saveUserStats(ctx context.Context, stats *UserStats) error {
	doc := map[string]interface{}{
		"_id":               stats.ID,
		"id":                stats.ID,
		"doc_type":          stats.DocType,
		"total_stats":       stats.TotalStats,
		"subspace_stats":    stats.SubspaceStats,
		"created_subspaces": stats.CreatedSubspaces,
		"joined_subspaces":  stats.JoinedSubspaces,
		"last_updated":      stats.LastUpdated,
	}

	if stats.VoteStats != nil {
		doc["vote_stats"] = stats.VoteStats
	}

	if stats.InviteStats != nil {
		doc["invite_stats"] = stats.InviteStats
	}

	_, err := um.db.Put(ctx, doc)
	return err
}

// QueryUsersBySubspace queries all users in a specific subspace
func (um *UserStatsManager) QueryUsersBySubspace(ctx context.Context, subspaceID string) ([]*UserStats, error) {
	var results []*UserStats

	queryFn := func(doc interface{}) (bool, error) {
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}

		// Check if it's a user statistics type
		docType, ok := docMap["doc_type"].(string)
		if !ok || docType != "user_stats" {
			return false, nil
		}

		// Check if it contains the specified subspace
		joinedSubspaces, ok := docMap["joined_subspaces"].([]interface{})
		if ok {
			for _, sid := range joinedSubspaces {
				if sidStr, ok := sid.(string); ok && sidStr == subspaceID {
					// Convert document to JSON
					jsonData, err := json.Marshal(docMap)
					if err != nil {
						return false, nil
					}

					var userStats UserStats
					if err := json.Unmarshal(jsonData, &userStats); err != nil {
						return false, nil
					}

					results = append(results, &userStats)
					return true, nil
				}
			}
		}

		return false, nil
	}

	// Execute query
	um.db.Query(ctx, queryFn)

	return results, nil
}

// QueryUserStats queries user statistics based on conditions
func (um *UserStatsManager) QueryUserStats(ctx context.Context, filter func(*UserStats) bool) ([]*UserStats, error) {
	var results []*UserStats

	queryFn := func(doc interface{}) (bool, error) {
		docMap, ok := doc.(map[string]interface{})
		if !ok {
			return false, nil
		}

		// Check if it's a user statistics type
		docType, ok := docMap["doc_type"].(string)
		if !ok || docType != "user_stats" {
			return false, nil
		}

		// Convert document to JSON
		jsonData, err := json.Marshal(docMap)
		if err != nil {
			return false, nil
		}

		var userStats UserStats
		if err := json.Unmarshal(jsonData, &userStats); err != nil {
			return false, nil
		}

		// Apply filter
		if filter == nil || filter(&userStats) {
			results = append(results, &userStats)
		}

		return true, nil
	}

	// Execute query
	um.db.Query(ctx, queryFn)

	return results, nil
}

// Helper function: check if a slice contains a string
func containsString(arr []string, str string) bool {
	for _, s := range arr {
		if s == str {
			return true
		}
	}
	return false
}
