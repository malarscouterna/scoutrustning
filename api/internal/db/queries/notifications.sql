-- name: LogNotification :exec
INSERT INTO notification_log (group_id, user_id, event_type, entity_id, channel, status, error, thread_key, message_id)
VALUES (@group_id, @user_id, @event_type, @entity_id, @channel, @status, @error, @thread_key, @message_id);

-- name: HasNotificationBeenSent :one
SELECT EXISTS (
    SELECT 1 FROM notification_log
    WHERE entity_id = @entity_id
      AND event_type = @event_type
      AND user_id = @user_id
      AND channel = @channel
      AND status = 'sent'
) AS sent;

-- name: GetThreadMessageID :one
-- Returns the stored email Message-ID for the first notification in a thread,
-- so follow-up sends can set In-Reply-To headers for email threading.
SELECT message_id FROM notification_log
WHERE thread_key = @thread_key
  AND user_id = @user_id
  AND channel = @channel
  AND message_id IS NOT NULL
ORDER BY created_at ASC
LIMIT 1;

-- name: GetGroupEnabledChannels :one
SELECT enabled_channels FROM group_settings WHERE group_id = @group_id;

-- name: GetBroadcastThreadMessageID :one
-- Returns the stored Message-ID for the first broadcast to a given thread+channel.
SELECT message_id FROM notification_log
WHERE thread_key = @thread_key
  AND channel = @channel
  AND user_id LIKE 'broadcast:%'
  AND message_id IS NOT NULL
ORDER BY created_at ASC
LIMIT 1;
