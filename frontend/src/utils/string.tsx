import { proxyImagePort } from "../config";

export const urlToBVID = (url: string) => {
  // 提取BV号的正则表达式
  const bvRegex = /BV[a-zA-Z0-9]+/;
  const match = url.match(bvRegex);

  return match ? match[0] : "";
};

export const bvidToUrl = (bvid: string) => {
  return `https://www.bilibili.com/video/${bvid}`;
};

export const convertToDuration = (seconds: number) => {
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;

  return `${minutes}:${remainingSeconds.toString().padStart(2, "0")}`;
};

export const graftingImage = (img: string) => {
  // if (img) {
  //     // 随机 0 到 3 中的一个数字
  //     let randomNum = Math.floor(Math.random() * 4);
  //     img = 'https://i' + randomNum + '.wp.com/' + img.replace('http://', '').replace('https://', '');
  //   }
  if (img) {
    return `http://127.0.0.1:${proxyImagePort}/image-proxy?url=${img}`;
  }

  return img;

  // return ProxyImage(img);
};

export const formatDate = (timestamp: number) => {
  const date = new Date(timestamp * 1000);
  return `${date.getFullYear()}年${String(date.getMonth() + 1).padStart(2, "0")}月${String(date.getDate()).padStart(2, "0")}日`;
};


export const formatDatetime = (timestamp: number) => {
  const date = new Date(timestamp * 1000);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
};