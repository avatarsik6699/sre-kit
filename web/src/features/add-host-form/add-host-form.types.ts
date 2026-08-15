export namespace AddHostFormTypes {
  export type Values = {
    label: string;
    address: string;
    sshPort: number;
    sshUser: string;
    sshKey: string;
  };

  export type Props = {
    isSubmitting: boolean;
    onSubmit: (values: Values) => void;
  };
}
