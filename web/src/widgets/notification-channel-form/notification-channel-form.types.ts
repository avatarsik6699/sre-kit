export namespace NotificationChannelFormTypes {
  export type Props = {
    isSubmitting: boolean;
    onSubmit: (input: { chatId: string; botToken: string }) => void;
  };
}
