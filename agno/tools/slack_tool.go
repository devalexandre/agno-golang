package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/slack-go/slack"
)

// SlackTool provides comprehensive integration with Slack API
// Supports messaging, channel management, user operations, file uploads, and more
type SlackTool struct {
	toolkit.Toolkit
	client *slack.Client
	token  string
}

// NewSlackTool creates a new Slack tool instance with the provided bot token
// The token should have appropriate OAuth scopes for the operations you want to perform
func NewSlackTool(token string) *SlackTool {
	if token == "" {
		panic("slack token is required")
	}

	tool := &SlackTool{
		Toolkit: toolkit.NewToolkit(),
		client:  slack.New(token),
		token:   token,
	}

	tool.Name = "slack"
	tool.Description = "Comprehensive Slack workspace integration - send messages, manage channels, handle threads, upload files, and more"

	// Register all methods with proper schemas
	tool.registerMethods()

	return tool
}

// registerMethods registers all available Slack operations
// registerMethods registers all available Slack operations
func (t *SlackTool) registerMethods() {
	// Message operations
	t.Register("sendMessage", "Send a new message to a Slack channel", t, t.sendMessage, SendMessageParams{})
	t.Register("sendThreadReply", "Send a reply in an existing message thread", t, t.sendThreadReply, SendThreadReplyParams{})
	t.Register("updateMessage", "Update the text of an existing message", t, t.updateMessage, UpdateMessageParams{})
	t.Register("deleteMessage", "Delete a message from a channel", t, t.deleteMessage, DeleteMessageParams{})

	// Channel operations
	t.Register("listChannels", "List all public and private channels in the workspace", t, t.listChannels, ListChannelsParams{})
	t.Register("getChannelInfo", "Get detailed information about a specific channel", t, t.getChannelInfo, GetChannelInfoParams{})
	t.Register("getChannelHistory", "Retrieve recent message history from a channel", t, t.getChannelHistory, GetChannelHistoryParams{})
	t.Register("createChannel", "Create a new public or private channel", t, t.createChannel, CreateChannelParams{})
	t.Register("archiveChannel", "Archive (deactivate) a channel", t, t.archiveChannel, ArchiveChannelParams{})
	t.Register("setChannelTopic", "Update the topic of a channel", t, t.setChannelTopic, SetChannelTopicParams{})
	t.Register("setChannelPurpose", "Update the purpose/description of a channel", t, t.setChannelPurpose, SetChannelPurposeParams{})

	// User operations
	t.Register("inviteToChannel", "Invite one or more users to a channel", t, t.inviteToChannel, InviteToChannelParams{})
	t.Register("removeFromChannel", "Remove a user from a channel", t, t.removeFromChannel, RemoveFromChannelParams{})
	t.Register("getUserInfo", "Get detailed profile information for a user", t, t.getUserInfo, GetUserInfoParams{})
	t.Register("listUsers", "List all active users in the workspace", t, t.listUsers, ListUsersParams{})
	t.Register("getUserPresence", "Check if a user is currently online or away", t, t.getUserPresence, GetUserPresenceParams{})

	// File operations
	t.Register("uploadFile", "Upload a file (text or binary) to Slack", t, t.uploadFile, UploadFileParams{})
	t.Register("listFiles", "List files shared in the workspace or a specific channel/user", t, t.listFiles, ListFilesParams{})
	t.Register("deleteFile", "Permanently delete a file by its ID", t, t.deleteFile, DeleteFileParams{})

	// Reaction operations
	t.Register("addReaction", "Add an emoji reaction to a message", t, t.addReaction, AddReactionParams{})
	t.Register("removeReaction", "Remove an emoji reaction from a message", t, t.removeReaction, RemoveReactionParams{})
	t.Register("getReactions", "List all reactions on a specific message", t, t.getReactions, GetReactionsParams{})

	// Search operations
	t.Register("searchMessages", "Search for messages matching a query across the workspace", t, t.searchMessages, SearchMessagesParams{})
	t.Register("searchFiles", "Search for files matching a query", t, t.searchFiles, SearchFilesParams{})

	// Thread operations
	t.Register("getThreadReplies", "Fetch all replies in a specific message thread", t, t.getThreadReplies, GetThreadRepliesParams{})

	// Pin operations
	t.Register("pinMessage", "Pin a message to the top of a channel", t, t.pinMessage, PinMessageParams{})
	t.Register("unpinMessage", "Unpin a previously pinned message", t, t.unpinMessage, UnpinMessageParams{})
	t.Register("listPins", "List all pinned items in a channel", t, t.listPins, ListPinsParams{})
}

// Parameter structs
type SendMessageParams struct {
	Channel string `json:"channel" description:"Channel ID or name"`
	Text    string `json:"text" description:"Message text"`
}

type ListChannelsParams struct {
	ExcludeArchived bool `json:"exclude_archived" description:"Exclude archived channels"`
	Limit           int  `json:"limit" description:"Number of channels to return"`
}

type GetChannelHistoryParams struct {
	Channel string `json:"channel" description:"Channel ID"`
	Limit   int    `json:"limit" description:"Number of messages to return"`
}

type CreateChannelParams struct {
	Name      string `json:"name" description:"Channel name"`
	IsPrivate bool   `json:"is_private" description:"Whether the channel is private"`
}

type InviteToChannelParams struct {
	Channel string   `json:"channel" description:"Channel ID"`
	Users   []string `json:"users" description:"Array of user IDs"`
}

type UploadFileParams struct {
	Channels string `json:"channels" description:"Channel ID"`
	Content  string `json:"content" description:"File content"`
	Filename string `json:"filename" description:"File name"`
	Title    string `json:"title" description:"File title"`
}

type AddReactionParams struct {
	Channel   string `json:"channel" description:"Channel ID"`
	Timestamp string `json:"timestamp" description:"Message timestamp"`
	Emoji     string `json:"emoji" description:"Emoji name (without colons)"`
}

type GetUserInfoParams struct {
	User string `json:"user" description:"User ID"`
}

type ListUsersParams struct{}

type SendThreadReplyParams struct {
	Channel  string `json:"channel" description:"Channel ID"`
	ThreadTS string `json:"thread_ts" description:"Thread timestamp"`
	Text     string `json:"text" description:"Reply text"`
}

type SearchMessagesParams struct {
	Query string `json:"query" description:"Search query"`
	Count int    `json:"count" description:"Number of results"`
}

type SetChannelTopicParams struct {
	Channel string `json:"channel" description:"Channel ID"`
	Topic   string `json:"topic" description:"Channel topic"`
}

type UpdateMessageParams struct {
	Channel   string `json:"channel" description:"Channel ID"`
	Timestamp string `json:"timestamp" description:"Message timestamp"`
	Text      string `json:"text" description:"New message text"`
}

type DeleteMessageParams struct {
	Channel   string `json:"channel" description:"Channel ID"`
	Timestamp string `json:"timestamp" description:"Message timestamp"`
}

type GetChannelInfoParams struct {
	Channel string `json:"channel" description:"Channel ID"`
}

type ArchiveChannelParams struct {
	Channel string `json:"channel" description:"Channel ID"`
}

type SetChannelPurposeParams struct {
	Channel string `json:"channel" description:"Channel ID"`
	Purpose string `json:"purpose" description:"Channel purpose"`
}

type RemoveFromChannelParams struct {
	Channel string `json:"channel" description:"Channel ID"`
	User    string `json:"user" description:"User ID to remove"`
}

type GetUserPresenceParams struct {
	User string `json:"user" description:"User ID"`
}

type ListFilesParams struct {
	Channel string `json:"channel" description:"Channel ID (optional)"`
	User    string `json:"user" description:"User ID (optional)"`
	Count   int    `json:"count" description:"Number of files to return"`
}

type DeleteFileParams struct {
	File string `json:"file" description:"File ID"`
}

type RemoveReactionParams struct {
	Channel   string `json:"channel" description:"Channel ID"`
	Timestamp string `json:"timestamp" description:"Message timestamp"`
	Emoji     string `json:"emoji" description:"Emoji name (without colons)"`
}

type GetReactionsParams struct {
	Channel   string `json:"channel" description:"Channel ID"`
	Timestamp string `json:"timestamp" description:"Message timestamp"`
}

type SearchFilesParams struct {
	Query string `json:"query" description:"Search query"`
	Count int    `json:"count" description:"Number of results"`
}

type GetThreadRepliesParams struct {
	Channel  string `json:"channel" description:"Channel ID"`
	ThreadTS string `json:"thread_ts" description:"Thread timestamp"`
}

type PinMessageParams struct {
	Channel   string `json:"channel" description:"Channel ID"`
	Timestamp string `json:"timestamp" description:"Message timestamp"`
}

type UnpinMessageParams struct {
	Channel   string `json:"channel" description:"Channel ID"`
	Timestamp string `json:"timestamp" description:"Message timestamp"`
}

type ListPinsParams struct {
	Channel string `json:"channel" description:"Channel ID"`
}

// sendMessage sends a message to a Slack channel
func (t *SlackTool) sendMessage(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	text, ok := args["text"].(string)
	if !ok {
		return "", fmt.Errorf("text is required")
	}

	options := []slack.MsgOption{
		slack.MsgOptionText(text, false),
	}

	// Add blocks if provided
	if blocksData, ok := args["blocks"]; ok {
		if blocksJSON, err := json.Marshal(blocksData); err == nil {
			var blocks slack.Blocks
			if err := json.Unmarshal(blocksJSON, &blocks); err == nil {
				options = append(options, slack.MsgOptionBlocks(blocks.BlockSet...))
			}
		}
	}

	// Add attachments if provided
	if attachmentsData, ok := args["attachments"]; ok {
		if attachmentsJSON, err := json.Marshal(attachmentsData); err == nil {
			var attachments []slack.Attachment
			if err := json.Unmarshal(attachmentsJSON, &attachments); err == nil {
				options = append(options, slack.MsgOptionAttachments(attachments...))
			}
		}
	}

	channelID, timestamp, err := t.client.PostMessageContext(ctx, channel, options...)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	result := map[string]interface{}{
		"success":   true,
		"channel":   channelID,
		"timestamp": timestamp,
		"message":   "Message sent successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// listChannels lists all channels in the workspace
func (t *SlackTool) listChannels(ctx context.Context, args map[string]interface{}) (string, error) {
	excludeArchived := true
	if val, ok := args["exclude_archived"].(bool); ok {
		excludeArchived = val
	}

	limit := 100
	if val, ok := args["limit"].(float64); ok {
		limit = int(val)
	}

	params := &slack.GetConversationsParameters{
		ExcludeArchived: excludeArchived,
		Limit:           limit,
		Types:           []string{"public_channel", "private_channel"},
	}

	channels, _, err := t.client.GetConversationsContext(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to list channels: %w", err)
	}

	var channelList []map[string]interface{}
	for _, channel := range channels {
		channelList = append(channelList, map[string]interface{}{
			"id":          channel.ID,
			"name":        channel.Name,
			"is_private":  channel.IsPrivate,
			"is_member":   channel.IsMember,
			"topic":       channel.Topic.Value,
			"purpose":     channel.Purpose.Value,
			"num_members": channel.NumMembers,
		})
	}

	result := map[string]interface{}{
		"success":  true,
		"channels": channelList,
		"count":    len(channelList),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// getChannelHistory retrieves message history from a channel
func (t *SlackTool) getChannelHistory(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	limit := 100
	if val, ok := args["limit"].(float64); ok {
		limit = int(val)
	}

	params := &slack.GetConversationHistoryParameters{
		ChannelID: channel,
		Limit:     limit,
	}

	history, err := t.client.GetConversationHistoryContext(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get channel history: %w", err)
	}

	var messages []map[string]interface{}
	for _, msg := range history.Messages {
		messages = append(messages, map[string]interface{}{
			"user":      msg.User,
			"text":      msg.Text,
			"timestamp": msg.Timestamp,
			"thread_ts": msg.ThreadTimestamp,
		})
	}

	result := map[string]interface{}{
		"success":  true,
		"messages": messages,
		"count":    len(messages),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// createChannel creates a new Slack channel
func (t *SlackTool) createChannel(ctx context.Context, args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok {
		return "", fmt.Errorf("name is required")
	}

	isPrivate := false
	if val, ok := args["is_private"].(bool); ok {
		isPrivate = val
	}

	params := slack.CreateConversationParams{
		ChannelName: name,
		IsPrivate:   isPrivate,
	}

	channel, err := t.client.CreateConversationContext(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to create channel: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"channel": map[string]interface{}{
			"id":         channel.ID,
			"name":       channel.Name,
			"is_private": channel.IsPrivate,
		},
		"message": fmt.Sprintf("Channel '%s' created successfully", name),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// inviteToChannel invites users to a channel
func (t *SlackTool) inviteToChannel(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	usersData, ok := args["users"]
	if !ok {
		return "", fmt.Errorf("users is required")
	}

	var users []string
	switch v := usersData.(type) {
	case []interface{}:
		for _, u := range v {
			if userStr, ok := u.(string); ok {
				users = append(users, userStr)
			}
		}
	case []string:
		users = v
	default:
		return "", fmt.Errorf("users must be an array of strings")
	}

	_, err := t.client.InviteUsersToConversationContext(ctx, channel, users...)
	if err != nil {
		return "", fmt.Errorf("failed to invite users: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Invited %d users to channel", len(users)),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// uploadFile uploads a file to Slack
func (t *SlackTool) uploadFile(ctx context.Context, args map[string]interface{}) (string, error) {
	channels, ok := args["channels"].(string)
	if !ok {
		return "", fmt.Errorf("channels is required")
	}

	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	filename := "file.txt"
	if val, ok := args["filename"].(string); ok {
		filename = val
	}

	params := slack.FileUploadParameters{
		Channels: []string{channels},
		Content:  content,
		Filename: filename,
	}

	if title, ok := args["title"].(string); ok {
		params.Title = title
	}

	if comment, ok := args["initial_comment"].(string); ok {
		params.InitialComment = comment
	}

	file, err := t.client.UploadFileContext(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"file": map[string]interface{}{
			"id":   file.ID,
			"name": file.Name,
			"url":  file.URLPrivate,
		},
		"message": "File uploaded successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// addReaction adds a reaction to a message
func (t *SlackTool) addReaction(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	timestamp, ok := args["timestamp"].(string)
	if !ok {
		return "", fmt.Errorf("timestamp is required")
	}

	emoji, ok := args["emoji"].(string)
	if !ok {
		return "", fmt.Errorf("emoji is required")
	}

	ref := slack.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	err := t.client.AddReactionContext(ctx, emoji, ref)
	if err != nil {
		return "", fmt.Errorf("failed to add reaction: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Added reaction :%s:", emoji),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// getUserInfo gets information about a user
func (t *SlackTool) getUserInfo(ctx context.Context, args map[string]interface{}) (string, error) {
	userID, ok := args["user"].(string)
	if !ok {
		return "", fmt.Errorf("user is required")
	}

	user, err := t.client.GetUserInfoContext(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user info: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"user": map[string]interface{}{
			"id":        user.ID,
			"name":      user.Name,
			"real_name": user.RealName,
			"email":     user.Profile.Email,
			"title":     user.Profile.Title,
			"is_bot":    user.IsBot,
			"is_admin":  user.IsAdmin,
			"is_owner":  user.IsOwner,
			"timezone":  user.TZ,
		},
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// listUsers lists all users in the workspace
func (t *SlackTool) listUsers(ctx context.Context, args map[string]interface{}) (string, error) {
	users, err := t.client.GetUsersContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list users: %w", err)
	}

	var userList []map[string]interface{}
	for _, user := range users {
		if user.Deleted {
			continue
		}

		userList = append(userList, map[string]interface{}{
			"id":        user.ID,
			"name":      user.Name,
			"real_name": user.RealName,
			"email":     user.Profile.Email,
			"is_bot":    user.IsBot,
		})
	}

	result := map[string]interface{}{
		"success": true,
		"users":   userList,
		"count":   len(userList),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// sendThreadReply sends a reply to a thread
func (t *SlackTool) sendThreadReply(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	threadTS, ok := args["thread_ts"].(string)
	if !ok {
		return "", fmt.Errorf("thread_ts is required")
	}

	text, ok := args["text"].(string)
	if !ok {
		return "", fmt.Errorf("text is required")
	}

	options := []slack.MsgOption{
		slack.MsgOptionText(text, false),
		slack.MsgOptionTS(threadTS),
	}

	_, timestamp, err := t.client.PostMessageContext(ctx, channel, options...)
	if err != nil {
		return "", fmt.Errorf("failed to send thread reply: %w", err)
	}

	result := map[string]interface{}{
		"success":   true,
		"timestamp": timestamp,
		"message":   "Thread reply sent successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// searchMessages searches for messages in the workspace
func (t *SlackTool) searchMessages(ctx context.Context, args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query is required")
	}

	count := 20
	if c, ok := args["count"].(float64); ok {
		count = int(c)
	}

	params := slack.SearchParameters{
		Count: count,
	}

	searchResult, err := t.client.SearchMessagesContext(ctx, query, params)
	if err != nil {
		return "", fmt.Errorf("failed to search messages: %w", err)
	}

	var messages []map[string]interface{}
	for _, match := range searchResult.Matches {
		messages = append(messages, map[string]interface{}{
			"text":      match.Text,
			"user":      match.Username,
			"channel":   match.Channel.Name,
			"timestamp": match.Timestamp,
		})
	}

	result := map[string]interface{}{
		"success":  true,
		"messages": messages,
		"count":    len(messages),
		"total":    searchResult.Total,
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// setChannelTopic sets the topic for a channel
func (t *SlackTool) setChannelTopic(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	topic, ok := args["topic"].(string)
	if !ok {
		return "", fmt.Errorf("topic is required")
	}

	_, err := t.client.SetTopicOfConversationContext(ctx, channel, topic)
	if err != nil {
		return "", fmt.Errorf("failed to set channel topic: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Channel topic updated successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// updateMessage updates an existing message
func (t *SlackTool) updateMessage(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	timestamp, ok := args["timestamp"].(string)
	if !ok {
		return "", fmt.Errorf("timestamp is required")
	}

	text, ok := args["text"].(string)
	if !ok {
		return "", fmt.Errorf("text is required")
	}

	_, _, _, err := t.client.UpdateMessageContext(ctx, channel, timestamp, slack.MsgOptionText(text, false))
	if err != nil {
		return "", fmt.Errorf("failed to update message: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Message updated successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// deleteMessage deletes a message
func (t *SlackTool) deleteMessage(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	timestamp, ok := args["timestamp"].(string)
	if !ok {
		return "", fmt.Errorf("timestamp is required")
	}

	_, _, err := t.client.DeleteMessageContext(ctx, channel, timestamp)
	if err != nil {
		return "", fmt.Errorf("failed to delete message: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Message deleted successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// getChannelInfo gets detailed information about a channel
func (t *SlackTool) getChannelInfo(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	info, err := t.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
		ChannelID: channel,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get channel info: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"channel": map[string]interface{}{
			"id":          info.ID,
			"name":        info.Name,
			"is_private":  info.IsPrivate,
			"is_archived": info.IsArchived,
			"is_member":   info.IsMember,
			"topic":       info.Topic.Value,
			"purpose":     info.Purpose.Value,
			"num_members": info.NumMembers,
			"created":     info.Created,
		},
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// archiveChannel archives a channel
func (t *SlackTool) archiveChannel(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	err := t.client.ArchiveConversationContext(ctx, channel)
	if err != nil {
		return "", fmt.Errorf("failed to archive channel: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Channel archived successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// setChannelPurpose sets the purpose for a channel
func (t *SlackTool) setChannelPurpose(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	purpose, ok := args["purpose"].(string)
	if !ok {
		return "", fmt.Errorf("purpose is required")
	}

	_, err := t.client.SetPurposeOfConversationContext(ctx, channel, purpose)
	if err != nil {
		return "", fmt.Errorf("failed to set channel purpose: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Channel purpose updated successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// removeFromChannel removes a user from a channel
func (t *SlackTool) removeFromChannel(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	user, ok := args["user"].(string)
	if !ok {
		return "", fmt.Errorf("user is required")
	}

	err := t.client.KickUserFromConversationContext(ctx, channel, user)
	if err != nil {
		return "", fmt.Errorf("failed to remove user from channel: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "User removed from channel successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// getUserPresence gets the presence status of a user
func (t *SlackTool) getUserPresence(ctx context.Context, args map[string]interface{}) (string, error) {
	user, ok := args["user"].(string)
	if !ok {
		return "", fmt.Errorf("user is required")
	}

	presence, err := t.client.GetUserPresenceContext(ctx, user)
	if err != nil {
		return "", fmt.Errorf("failed to get user presence: %w", err)
	}

	result := map[string]interface{}{
		"success":   true,
		"presence":  presence.Presence,
		"online":    presence.Online,
		"auto_away": presence.AutoAway,
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// listFiles lists files in the workspace
func (t *SlackTool) listFiles(ctx context.Context, args map[string]interface{}) (string, error) {
	params := slack.GetFilesParameters{
		Count: 20,
	}

	if channel, ok := args["channel"].(string); ok && channel != "" {
		params.Channel = channel
	}

	if user, ok := args["user"].(string); ok && user != "" {
		params.User = user
	}

	if count, ok := args["count"].(float64); ok {
		params.Count = int(count)
	}

	files, _, err := t.client.GetFilesContext(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to list files: %w", err)
	}

	var fileList []map[string]interface{}
	for _, file := range files {
		fileList = append(fileList, map[string]interface{}{
			"id":       file.ID,
			"name":     file.Name,
			"title":    file.Title,
			"mimetype": file.Mimetype,
			"size":     file.Size,
			"url":      file.URLPrivate,
			"user":     file.User,
		})
	}

	result := map[string]interface{}{
		"success": true,
		"files":   fileList,
		"count":   len(fileList),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// deleteFile deletes a file
func (t *SlackTool) deleteFile(ctx context.Context, args map[string]interface{}) (string, error) {
	fileID, ok := args["file"].(string)
	if !ok {
		return "", fmt.Errorf("file is required")
	}

	err := t.client.DeleteFileContext(ctx, fileID)
	if err != nil {
		return "", fmt.Errorf("failed to delete file: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "File deleted successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// removeReaction removes a reaction from a message
func (t *SlackTool) removeReaction(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	timestamp, ok := args["timestamp"].(string)
	if !ok {
		return "", fmt.Errorf("timestamp is required")
	}

	emoji, ok := args["emoji"].(string)
	if !ok {
		return "", fmt.Errorf("emoji is required")
	}

	ref := slack.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	err := t.client.RemoveReactionContext(ctx, emoji, ref)
	if err != nil {
		return "", fmt.Errorf("failed to remove reaction: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Removed reaction :%s:", emoji),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// getReactions gets all reactions for a message
func (t *SlackTool) getReactions(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	timestamp, ok := args["timestamp"].(string)
	if !ok {
		return "", fmt.Errorf("timestamp is required")
	}

	ref := slack.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	reactions, err := t.client.GetReactionsContext(ctx, ref, slack.GetReactionsParameters{})
	if err != nil {
		return "", fmt.Errorf("failed to get reactions: %w", err)
	}

	var reactionList []map[string]interface{}
	for _, reaction := range reactions {
		reactionList = append(reactionList, map[string]interface{}{
			"name":  reaction.Name,
			"count": reaction.Count,
			"users": reaction.Users,
		})
	}

	result := map[string]interface{}{
		"success":   true,
		"reactions": reactionList,
		"count":     len(reactionList),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// searchFiles searches for files in the workspace
func (t *SlackTool) searchFiles(ctx context.Context, args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query is required")
	}

	count := 20
	if c, ok := args["count"].(float64); ok {
		count = int(c)
	}

	params := slack.SearchParameters{
		Count: count,
	}

	searchResult, err := t.client.SearchFilesContext(ctx, query, params)
	if err != nil {
		return "", fmt.Errorf("failed to search files: %w", err)
	}

	var files []map[string]interface{}
	for _, match := range searchResult.Matches {
		files = append(files, map[string]interface{}{
			"id":       match.ID,
			"name":     match.Name,
			"title":    match.Title,
			"mimetype": match.Mimetype,
			"size":     match.Size,
			"url":      match.URLPrivate,
		})
	}

	result := map[string]interface{}{
		"success": true,
		"files":   files,
		"count":   len(files),
		"total":   searchResult.Total,
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// getThreadReplies gets all replies in a thread
func (t *SlackTool) getThreadReplies(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	threadTS, ok := args["thread_ts"].(string)
	if !ok {
		return "", fmt.Errorf("thread_ts is required")
	}

	params := &slack.GetConversationRepliesParameters{
		ChannelID: channel,
		Timestamp: threadTS,
	}

	messages, _, _, err := t.client.GetConversationRepliesContext(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to get thread replies: %w", err)
	}

	var replies []map[string]interface{}
	for _, msg := range messages {
		replies = append(replies, map[string]interface{}{
			"user":      msg.User,
			"text":      msg.Text,
			"timestamp": msg.Timestamp,
		})
	}

	result := map[string]interface{}{
		"success": true,
		"replies": replies,
		"count":   len(replies),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// pinMessage pins a message to a channel
func (t *SlackTool) pinMessage(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	timestamp, ok := args["timestamp"].(string)
	if !ok {
		return "", fmt.Errorf("timestamp is required")
	}

	ref := slack.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	err := t.client.AddPinContext(ctx, channel, ref)
	if err != nil {
		return "", fmt.Errorf("failed to pin message: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Message pinned successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// unpinMessage unpins a message from a channel
func (t *SlackTool) unpinMessage(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	timestamp, ok := args["timestamp"].(string)
	if !ok {
		return "", fmt.Errorf("timestamp is required")
	}

	ref := slack.ItemRef{
		Channel:   channel,
		Timestamp: timestamp,
	}

	err := t.client.RemovePinContext(ctx, channel, ref)
	if err != nil {
		return "", fmt.Errorf("failed to unpin message: %w", err)
	}

	result := map[string]interface{}{
		"success": true,
		"message": "Message unpinned successfully",
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// listPins lists all pinned messages in a channel
func (t *SlackTool) listPins(ctx context.Context, args map[string]interface{}) (string, error) {
	channel, ok := args["channel"].(string)
	if !ok {
		return "", fmt.Errorf("channel is required")
	}

	pins, _, err := t.client.ListPinsContext(ctx, channel)
	if err != nil {
		return "", fmt.Errorf("failed to list pins: %w", err)
	}

	var pinList []map[string]interface{}
	for _, pin := range pins {
		if pin.Message != nil {
			pinList = append(pinList, map[string]interface{}{
				"type":      "message",
				"user":      pin.Message.User,
				"text":      pin.Message.Text,
				"timestamp": pin.Message.Timestamp,
			})
		}
	}

	result := map[string]interface{}{
		"success": true,
		"pins":    pinList,
		"count":   len(pinList),
	}

	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

// GetJSONSchema returns the JSON schema for the Slack tool
func (t *SlackTool) GetJSONSchema() interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type": "string",
				"enum": []string{
					// Message operations
					"send_message",
					"send_thread_reply",
					"update_message",
					"delete_message",
					// Channel operations
					"list_channels",
					"get_channel_info",
					"get_channel_history",
					"create_channel",
					"archive_channel",
					"set_channel_topic",
					"set_channel_purpose",
					// User operations
					"invite_to_channel",
					"remove_from_channel",
					"get_user_info",
					"list_users",
					"get_user_presence",
					// File operations
					"upload_file",
					"list_files",
					"delete_file",
					// Reaction operations
					"add_reaction",
					"remove_reaction",
					"get_reactions",
					// Search operations
					"search_messages",
					"search_files",
					// Thread operations
					"get_thread_replies",
					// Pin operations
					"pin_message",
					"unpin_message",
					"list_pins",
				},
				"description": "The action to perform",
			},
			"channel": map[string]interface{}{
				"type":        "string",
				"description": "Channel ID or name (required for most actions)",
			},
			"text": map[string]interface{}{
				"type":        "string",
				"description": "Message text",
			},
			"user": map[string]interface{}{
				"type":        "string",
				"description": "User ID",
			},
			"users": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Array of user IDs",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Channel name",
			},
			"is_private": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the channel is private",
			},
			"limit": map[string]interface{}{
				"type":        "number",
				"description": "Number of items to return",
			},
			"timestamp": map[string]interface{}{
				"type":        "string",
				"description": "Message timestamp",
			},
			"thread_ts": map[string]interface{}{
				"type":        "string",
				"description": "Thread timestamp",
			},
			"emoji": map[string]interface{}{
				"type":        "string",
				"description": "Emoji name (without colons)",
			},
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query",
			},
			"topic": map[string]interface{}{
				"type":        "string",
				"description": "Channel topic",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "File content",
			},
			"filename": map[string]interface{}{
				"type":        "string",
				"description": "File name",
			},
		},
		"required": []string{"action"},
	}
}
