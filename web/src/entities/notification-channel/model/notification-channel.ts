// NotificationChannel domain type + CRUD hooks over /api/notification-channels (docs/SPEC.md
// §3/§4). type is "telegram" only in v1; bot_token is write-only (never returned by the API).
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiClient, normalizeApiFailure } from "~/shared/api";
import type { components } from "~/shared/api";

type NotificationChannelResponse =
  components["schemas"]["internal_alertrouter_interfaces_http.notificationChannelResponse"];

export type NotificationChannel = {
  id: string;
  type: string;
  chatId: string;
  enabled: boolean;
};

function toNotificationChannel(
  raw: NotificationChannelResponse,
): NotificationChannel {
  return {
    id: raw.id ?? "",
    type: raw.type ?? "telegram",
    chatId: raw.chat_id ?? "",
    enabled: raw.enabled ?? false,
  };
}

export const notificationChannelsQueryKey = ["notification-channels"] as const;

export function useNotificationChannelsQuery() {
  return useQuery({
    queryKey: notificationChannelsQueryKey,
    queryFn: async () => {
      const result = await apiClient.GET("/api/notification-channels");
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return (result.data ?? []).map(toNotificationChannel);
    },
  });
}

export function useCreateNotificationChannelMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: {
      type: string;
      chatId: string;
      botToken: string;
    }) => {
      const result = await apiClient.POST("/api/notification-channels", {
        body: {
          type: input.type,
          chat_id: input.chatId,
          bot_token: input.botToken,
        },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return toNotificationChannel(result.data ?? {});
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: notificationChannelsQueryKey,
      });
    },
  });
}

export function useUpdateNotificationChannelMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: { id: string; enabled: boolean }) => {
      const result = await apiClient.PATCH("/api/notification-channels/{id}", {
        params: { path: { id: input.id } },
        body: { enabled: input.enabled },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
      return toNotificationChannel(result.data ?? {});
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: notificationChannelsQueryKey,
      });
    },
  });
}

export function useDeleteNotificationChannelMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      const result = await apiClient.DELETE("/api/notification-channels/{id}", {
        params: { path: { id } },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error, result.response.status);
      }
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: notificationChannelsQueryKey,
      });
    },
  });
}
