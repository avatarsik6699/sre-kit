import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { Button, PasswordInput, Stack } from "~/shared/ui";
import { apiClient, normalizeApiFailure } from "~/shared/api";
import { Typography } from "~/shared/components/typography";
import type { LoginFormTypes } from "./login-form.types";

function useLoginMutation() {
  return useMutation({
    mutationFn: async (password: string) => {
      const result = await apiClient.POST("/api/auth/login", {
        body: { password },
      });
      if (result.error) {
        throw normalizeApiFailure(result.error);
      }
    },
  });
}

/** Admin-password login form (docs/SPEC.md §6) — the session cookie is set by the server; this
 * component only reports success/failure. */
export const LoginForm: React.FC<LoginFormTypes.Props> = (props) => {
  const [password, setPassword] = useState("");
  const loginMutation = useLoginMutation();

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    loginMutation.mutate(password, { onSuccess: props.onSuccess });
  }

  return (
    <form onSubmit={handleSubmit}>
      <Stack gap="sm" w={320}>
        <Typography variant="title" order={2}>
          sre-kit
        </Typography>
        <PasswordInput
          label="Admin password"
          value={password}
          onChange={(event) => setPassword(event.currentTarget.value)}
          autoFocus
          required
        />
        {loginMutation.isError ? (
          <Typography c="statusCritical">
            {normalizeApiFailure(loginMutation.error).message}
          </Typography>
        ) : null}
        <Button type="submit" loading={loginMutation.isPending} fullWidth>
          Log in
        </Button>
      </Stack>
    </form>
  );
};
