import type { ImageTypes } from "./image.types";

/** The only place a raw `<img>` (or repository-owned's Image) may render — ESLint forbids both elsewhere. */
export const Image: React.FC<ImageTypes.Props> = (props) => {
  return (
    <img
      src={props.src}
      alt={props.alt}
      width={props.width}
      height={props.height}
      className={props.className}
      style={{ borderRadius: props.radius }}
    />
  );
};
