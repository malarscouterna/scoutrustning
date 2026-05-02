-- name: LogNotification :exec
INSERT INTO notification_log (group_id, user_id, event_type, entity_id, channel, status, error)
VALUES (@group_id, @user_id, @event_type, @entity_id, @channel, @status, @error);

-- name: HasNotificationBeenSent :one
SELECT EXISTS (
    SELECT 1 FROM notification_log
    WHERE entity_id = @entity_id
      AND event_type = @event_type
      AND user_id = @user_id
      AND channel = @channel
      AND status = 'sent'
) AS sent;
