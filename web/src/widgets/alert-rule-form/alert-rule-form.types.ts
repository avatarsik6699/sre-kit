import type { AlertRuleInput } from "~/entities/alert";
import type { NotificationChannel } from "~/entities/notification-channel";
import type { Source } from "~/entities/source";

export namespace AlertRuleFormTypes {
  export type Props = {
    sources: Source[];
    channels: NotificationChannel[];
    isSubmitting: boolean;
    onSubmit: (input: AlertRuleInput) => void;
  };
}
