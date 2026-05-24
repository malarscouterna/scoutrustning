# Google Chat Integration — Manager Setup Guide

This guide walks a scout group manager through connecting the equipment system to Google Chat so booking and issue notifications are posted to your team's chat spaces.

**Requirements**: Your scout group must use Google Workspace (formerly G Suite). Groups on personal Gmail accounts cannot use this integration. You are assumed to use ScoutIT integration and [Google-Scoutnet-synk](https://github.com/Scouterna/Google-Scoutnet-synk), though this might not be mandatory.

---

## Overview

Once connected, the system can post booking and issue notifications as threaded cards directly into a Google Chat Space (e.g. your troop's chat room). All messages for the same booking are grouped in one thread, keeping your space tidy.

The setup has two parts:

1. **Create a service account** in Google Cloud — this is the "bot" identity that sends messages.
2. **Connect it in the equipment system** and map your teams to Chat spaces.

---

## Part 1 — Create a service account in Google Cloud

You need access to the **Google Cloud Console** and the **Google Workspace Admin Console**. If your group has a dedicated IT contact, they may need to do Part 1 on your behalf.

### 1.1 Create or use a Google Cloud project

1. Go to [console.cloud.google.com](https://console.cloud.google.com).
2. Select your organisation from the top dropdown, then create a new project (e.g. "Scout Notifications") or use an existing one. If creating a new one, make sure to select it after creation.

### 1.2 Enable the Google Chat API

1. In the Cloud Console, open **APIs & Services > Library**.
2. Search for **Google Chat API** and click **Enable**.

### 1.3 Create a service account

1. Go to **IAM & Admin > Service Accounts**.
2. Click **Create Service Account**.
3. Name it something descriptive, e.g. `scout-notifications`.
4. Click through the optional Permissions and Principals with access steps (no special IAM role is needed for Chat).
5. Open the new service account, go to the **Keys** tab, click **Add Key > Create new key**, choose **JSON**, and download the file. Keep this file safe — it contains credentials.

### 1.4 Enable Domain-Wide Delegation

1. Still on the service account page, click **Details** and expand **Advanced settings**.
2. Note the **Client ID** shown under **Domain-wide Delegation** — you will need it in step 1.6.

### 1.5 Configure a Google Chat app

1. In the Cloud Console, go to **APIs & Services > Google Chat API > Configuration**.
2. Fill in at minimum:
   - **App name**: e.g. `Scoutrustning`
   - **Avatar URL**: any square image URL, we recommend the [MS Utrustning logo](/PNG%20Utrustningsgruppen%20-%20Logotyp.png) TODO UPDATE TO GENERAL IMAGE
   - **Description**: e.g. `Bokningar och felanmälningar`
3. Under **Functionality**, enable **Join spaces and group conversations**.
4. Under **Connection settings**, select **HTTP endpoint URL** and enter any placeholder URL for now (the bot does not receive incoming messages).
5. Under **Visibility**, publish the app to your Workspace domain so members can find it:
   - Select **Publish to domain** (not "Publish to public").
   - Click the link **Publish app** that appears, which takes you to the Google Workspace Marketplace SDK page.
   - On the **Store listing** tab, fill in the required fields (app name, description, support email). These are only shown inside your organisation.
   - Click **Publish**. The app is now searchable for everyone in your Workspace domain. This step is required so that members can manually add the bot to a space (see section 2.3 below).
6. Click **Save**.

### 1.6 Grant the Chat API scope in the Admin Console

1. Go to [admin.google.com](https://admin.google.com).
2. Navigate to **Security > Access and data control > API controls > Domain-wide delegation**.
3. Click **Add new**.
4. Enter the **Client ID** from step 1.4.
5. In the **OAuth scopes** field, enter exactly (one per line or comma-separated):
   ```
   https://www.googleapis.com/auth/chat.spaces.readonly,https://www.googleapis.com/auth/chat.memberships.app
   ```
6. Click **Authorise**.

---

## Part 2 — Connect in the equipment system

### 2.1 Upload the service account key

1. In the equipment system, go to your **Profile** and open the **Group Settings** tab.
2. Scroll to the **Integrationer** section.
3. Upload the JSON key file you downloaded in step 1.3.
4. Click **Anslut Google Chat**.
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
2. Click **Länka**. The system adds the bot to the space (acting as the admin account) and posts a welcome message.

The dropdown has two groups:
- **Bot already added — link directly**: the bot is already a member of this space (e.g. added manually or from a previous link). Just select and click **Länka**.
- **Add automatically**: the admin account is a member of this space, so the system can add the bot on your behalf when you click **Länka**.

> **Note**: the admin account entered in step 2.1 must be a **member of each space** you want to link for automatic add to work. If a space is missing from the dropdown, use the manual method below.

### 2.3 Manually adding the bot to a space

If a space does not appear in the dropdown (e.g. the admin account is not a member), you can add the bot directly from Google Chat:

1. Open the space in Google Chat.
2. Click the space name at the top to open its settings.
3. Go to **Apps & integrations** → **Add apps**.
4. Search for your app name (e.g. `Scoutrustning`) and click **Add**.
5. Return to the equipment system and reload the **Group Settings** page.
6. The space will now appear in the dropdown under **Bot already added — link directly**. Select it and click **Länka**.

> The app must be published to your domain (step 1.5) before it is searchable in step 4.

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
