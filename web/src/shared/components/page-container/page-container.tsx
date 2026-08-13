import { Container } from "@mantine/core";
import type { PageContainerTypes } from "./page-container.types";

/** The only place Mantine's Container may be used directly — ESLint forbids it elsewhere. */
export const PageContainer: React.FC<PageContainerTypes.Props> = (props) => {
  return (
    <Container size={props.size ?? "xl"} className={props.className}>
      {props.children}
    </Container>
  );
};
