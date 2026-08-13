export namespace ImageTypes {
  export type Props = {
    src: string;
    /** Required — no decorative-only escape hatch; this is a compact SRE tool, every image carries meaning. */
    alt: string;
    width?: number | string;
    height?: number | string;
    radius?: string | number;
    className?: string;
  };
}
