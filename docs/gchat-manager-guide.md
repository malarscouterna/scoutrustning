# Google Chat Integration — Manager Setup Guide

> **Preliminary draft.** Written before implementation as a planning aid. Review and update after Phase 3.5b ships.

This guide walks a scout group manager through connecting the equipment system to Google Chat so booking and issue notifications are posted to your team's chat spaces.

**Requirements**: Your scout group must use Google Workspace (formerly G Suite). Groups on personal Gmail accounts cannot use this integration.

---

## Overview

Once connected, the system can post booking and issue notifications as threaded cards directly into a Google Chat Space (e.g. your patrol's chat room). All messages for the same booking are grouped in one thread, keeping your space tidy.

The setup has two parts:

1. **Create a service account** in Google Cloud — this is the "bot" identity that sends messages.
2. **Connect it in the equipment system** and map your teams to Chat spaces.

---

## Part 1 — Create a service account in Google Cloud

You need access to the **Google Cloud Console** and the **Google Workspace Admin Console**. If your group has a dedicated IT contact, they may need to do Part 1 on your behalf.

### 1.1 Create or use a Google Cloud project

1. Go to [console.cloud.google.com](https://console.cloud.google.com).
2. Select your organisation from the top dropdown, then create a new project (e.g. "Scout Notifications") or use an existing one.

### 1.2 Enable the Google Chat API

1. In the Cloud Console, open **APIs & Services > Library**.
2. Search for **Google Chat API** and click **Enable**.

### 1.3 Create a service account

1. Go to **IAM & Admin > Service Accounts**.
2. Click **Create Service Account**.
3. Name it something descriptive, e.g. `scout-notifications`.
4. Click through the optional role and user steps (no special IAM role is needed for Chat).
5. Open the new service account, go to the **Keys** tab, click **Add Key > Create new key**, choose **JSON**, and download the file. Keep this file safe — it contains credentials.

### 1.4 Enable Domain-Wide Delegation

1. Still on the service account page, click **Edit** (pencil icon).
2. Expand **Advanced settings** (or scroll to "Domain-Wide Delegation").
3. Check **Enable Google Workspace Domain-Wide Delegation**.
4. Note the **Client ID** shown — you will need it in step 1.6.

### 1.5 Configure a Google Chat app

1. In the Cloud Console, go to **APIs & Services > Google Chat API > Configuration**.
2. Fill in at minimum:
   - **App name**: e.g. `Utrustningsregistret`
   - **Avatar URL**: any square image URL (can be updated later)
   - **Description**: e.g. `Booking and issue notifications`
3. Under **Functionality**, enable **Join spaces and group conversations**.
4. Under **Connection settings**, select **App URL** and enter any placeholder URL for now (the bot does not receive incoming messages).
5. Set **Visibility** to your Workspace domain (not public).
6. Click **Save**.

### 1.6 Grant the Chat API scope in the Admin Console

1. Go to [admin.google.com](https://admin.google.com).
2. Navigate to **Security > Access and data control > API controls > Domain-wide delegation**.
3. Click **Add new**.
4. Enter the **Client ID** from step 1.4.
5. In the **OAuth scopes** field, enter exactly:
   ```
   https://www.googleapis.com/auth/chat.bot
   ```
6. Click **Authorise**.

---

## Part 2 — Connect in the equipment system

### 2.1 Upload the service account key

1. In the equipment system, go to your **Profile** and open the **Group Settings** tab.
2. Scroll to the **Integrationer** section.
3. Click **Anslut Google Chat**.
4. Upload the JSON key file you downloaded in step 1.3.
5. Enter the email address of a Google Workspace admin in your organisation (e.g. `admin@yourgroup.se`). The bot will impersonate this account to list and join spaces. The account needs no special permissions beyond being a Workspace admin.
6. Click **Anslut**. The system will verify the credentials. If it succeeds, you'll see a list of your organisation's Chat spaces.

If the verification fails, the most common causes are:
- The Chat API is not enabled on the Cloud project.
- The Domain-Wide Delegation Client ID or scope is wrong.
- The admin email is not a Workspace admin.

### 2.2 Map teams to Chat spaces

Once connected, a **Team mapper** table appears showing all your scout teams.

For each team you want to receive notifications:

1. Open the dropdown in the **Google Chat Space** column and select the space (e.g. "Örnarnas kår").
2. Click **Länka**. The bot is automatically added to the space and posts a welcome message.

Only spaces visible to the service account are listed. If a space is missing, check that the Workspace admin account has access to it.

### 2.3 Configure what gets sent

After linking, open the team's settings page (from the team list) and go to the **Notiser** section.

- **Broadcast to space**: the toggle table controls which event types are sent to the Chat space.
- **Suppress individual emails**: by default, team members also receive personal email notifications. Toggle "Skicka inte notiser till enskilda medlemmar som standard" if you prefer the Chat space to be the primary channel and want to reduce inbox noise. Members can still re-enable personal emails from their own profile.

Group-wide defaults (for all teams that haven't set their own preferences) are in **Group Settings > Standardinställningar för notiser**.

---

## Disconnecting

To remove the integration:

1. Go to **Group Settings > Integrationer**.
2. Click **Koppla bort Google Chat**.

This removes the service account key, stops all Chat notifications, and removes the space mappings. The bot will remain in the spaces — remove it manually from each space if needed via the space settings in Google Chat.

---

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| Upload fails with "invalid credentials" | Wrong JSON file, or the service account key has been revoked |
| Upload fails with "cannot list spaces" | DWD not configured, wrong Client ID, or scope not authorised in Admin Console |
| Space not in the dropdown | Admin account does not have access to the space |
| Bot joined but no messages appear | Check that the correct event types are enabled in the team's Notiser section |
| Messages appear twice | Team email broadcast and Chat space are both enabled, and the shared `notification_email` address is also in the space — disable one |
