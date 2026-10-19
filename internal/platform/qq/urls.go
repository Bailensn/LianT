package qq

import (
	"fmt"
	"os"
	"strings"
)

// QQ 开放平台地址；同时兼容旧 qbot 的 QBOT_* 变量，方便从 qbot 迁移。
func apiBase() string {
	if v := strings.TrimSpace(os.Getenv("LIANT_QQ_API_BASE")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(os.Getenv("QBOT_API_BASE")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "https://api.bot.qq.com"
}

func envPath(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func TokenURL() string {
	if v := strings.TrimSpace(os.Getenv("LIANT_QQ_TOKEN_URL")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("QBOT_TOKEN_URL")); v != "" {
		return v
	}
	return "https://bots.qq.com/app/getAppAccessToken"
}

func GatewayURL() string {
	return apiBase() + envPath("LIANT_QQ_GATEWAY_PATH", envPath("QBOT_GATEWAY_PATH", "/gateway"))
}

func GroupMessageURL(groupOpenID string) string {
	return apiBase() + fmt.Sprintf(envPath("LIANT_QQ_GROUP_MESSAGE_PATH", envPath("QBOT_GROUP_MESSAGE_PATH", "/v2/groups/%s/messages")), groupOpenID)
}

func GroupRecallURL(groupOpenID, messageID string) string {
	return apiBase() + fmt.Sprintf(envPath("LIANT_QQ_GROUP_RECALL_PATH", envPath("QBOT_GROUP_RECALL_PATH", "/v2/groups/%s/messages/%s")), groupOpenID, messageID)
}

func GroupMuteURL(groupOpenID string) string {
	return apiBase() + fmt.Sprintf(envPath("LIANT_QQ_GROUP_MUTE_PATH", envPath("QBOT_GROUP_MUTE_PATH", "/v2/groups/%s/restrict_chat_setting")), groupOpenID)
}

func GroupInfoURL(groupOpenID string) string {
	return apiBase() + fmt.Sprintf(envPath("LIANT_QQ_GROUP_INFO_PATH", envPath("QBOT_GROUP_INFO_PATH", "/v2/groups/%s/info")), groupOpenID)
}

func GroupBotStateURL(groupOpenID string) string {
	return apiBase() + fmt.Sprintf(envPath("LIANT_QQ_GROUP_BOT_STATE_PATH", envPath("QBOT_GROUP_BOT_STATE_PATH", "/v2/groups/%s/bot_state")), groupOpenID)
}

func GroupMembersURL(groupOpenID string) string {
	return apiBase() + fmt.Sprintf(envPath("LIANT_QQ_GROUP_MEMBERS_PATH", envPath("QBOT_GROUP_MEMBERS_PATH", "/v2/groups/%s/members")), groupOpenID)
}

func GroupRemoveMembersURL(groupOpenID string) string {
	return apiBase() + fmt.Sprintf(envPath("LIANT_QQ_GROUP_REMOVE_MEMBERS_PATH", envPath("QBOT_GROUP_REMOVE_MEMBERS_PATH", "/v2/groups/%s/batch_remove_members")), groupOpenID)
}

func GroupJoinListURL(groupOpenID string) string {
	return apiBase() + fmt.Sprintf(envPath("LIANT_QQ_GROUP_JOIN_LIST_PATH", envPath("QBOT_GROUP_JOIN_LIST_PATH", "/v2/groups/%s/join_request_list")), groupOpenID)
}

func GroupApproveJoinURL(groupOpenID, memberOpenID string) string {
	return apiBase() + fmt.Sprintf(envPath("LIANT_QQ_GROUP_APPROVE_JOIN_PATH", envPath("QBOT_GROUP_APPROVE_JOIN_PATH", "/v2/groups/%s/approval_join_request/%s")), groupOpenID, memberOpenID)
}

func GroupBlacklistURL(groupOpenID string) string {
	return apiBase() + fmt.Sprintf(envPath("LIANT_QQ_GROUP_BLACKLIST_PATH", envPath("QBOT_GROUP_BLACKLIST_PATH", "/v2/groups/%s/member_blacklist")), groupOpenID)
}

func C2cMessageURL(userOpenID string) string {
	return apiBase() + fmt.Sprintf(envPath("LIANT_QQ_C2C_MESSAGE_PATH", envPath("QBOT_C2C_MESSAGE_PATH", "/v2/users/%s/messages")), userOpenID)
}
