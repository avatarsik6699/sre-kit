import { Image as MantineImage } from "@mantine/core";
import type { ImageTypes } from "./image.types";

/** The only place a raw `<img>` (or Mantine's Image) may render — ESLint forbids both elsewhere. */
export const Image: React.FC<ImageTypes.Props> = (props) => {
  return (
    <MantineImage
      src={props.src}
      alt={props.alt}
      w={props.width}
      h={props.height}
      radius={props.radius}
      className={props.className}
    />
  );
};
