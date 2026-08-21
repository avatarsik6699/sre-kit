import { Center } from "~/shared/ui";
import { useNavigate } from "@tanstack/react-router";
import { LoginForm } from "~/features/login-form";

/** Route-level composition for /login (docs/SPEC.md §5.1). */
export const LoginPage: React.FC = () => {
  const navigate = useNavigate();

  return (
    <Center mih="100vh">
      <LoginForm onSuccess={() => void navigate({ to: "/" })} />
    </Center>
  );
};
