"use client";

import { useState, type CSSProperties } from "react";
import Image from "next/image";

type MachineryImageProps = {
  imagePath?: string;
  productName: string;
  width: number;
  height: number;
  style?: CSSProperties;
};

export function MachineryImage({ imagePath, productName, width, height, style }: MachineryImageProps) {
  const [imageFailed, setImageFailed] = useState(false);
  const shouldShowImage = Boolean(imagePath) && !imageFailed;

  if (!shouldShowImage) {
    return <p style={{ margin: 0, color: "#64748b" }}>Catalog image pending extraction</p>;
  }

  return (
    <Image
      src={imagePath as string}
      alt={`${productName} catalog page`}
      width={width}
      height={height}
      style={style}
      onError={() => setImageFailed(true)}
    />
  );
}
