"use client";

import { useState, type CSSProperties } from "react";
import Image from "next/image";

type MachineryImageProps = {
  imagePath?: string | null;
  imageStatus?: "ready" | "pending_extraction";
  productName: string;
  width: number;
  height: number;
  style?: CSSProperties;
};

export function MachineryImage({ imagePath, imageStatus, productName, width, height, style }: MachineryImageProps) {
  const [imageFailed, setImageFailed] = useState(false);
  const shouldShowImage = imageStatus === "ready" && Boolean(imagePath) && !imageFailed;

  if (!shouldShowImage) {
    return <p style={{ margin: 0, color: "#64748b" }}>Makine görseli hazırlanıyor</p>;
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
