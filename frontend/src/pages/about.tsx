import { useState, useEffect } from "react";
import { Card, CardBody, CardHeader, Chip, Link, Divider } from "@heroui/react";
import { Github, Info } from "@icon-park/react";
import { GetAppVersion } from "../../wailsjs/go/service/BL";
import DefaultLayout from "@/layouts/default";

interface AppVersion {
  version: string;
  build: number;
}

export default function AboutPage() {
  const [appVersion, setAppVersion] = useState<AppVersion | null>(null);

  useEffect(() => {
    GetAppVersion().then(setAppVersion).catch(console.error);
  }, []);

  const features = [
    "输入关键词搜索视频",
    "登录后查看订阅、收藏、推荐",
    "点击 UP 主查看作品列表",
    "支持点赞和投币",
    "系统媒体控制支持",
    "迷你模式",
  ];

  return (
    <DefaultLayout>
      <div className="max-w-4xl mx-auto py-8 space-y-8">
        <div className="text-center space-y-4">
          <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
            关于 bili-FM
          </h1>
          <p className="text-lg text-gray-600 dark:text-gray-400 max-w-2xl mx-auto">
            通过音频来听 B 站节目，你可以把它作为一个音乐播放器，也可以用来作为知识学习的工具。
          </p>
          <div className="flex justify-center gap-4">
            <Chip color="primary" variant="flat" size="lg">
              版本: {appVersion?.version ?? "..."}
            </Chip>
            <Chip color="secondary" variant="flat" size="lg">
              Build: {appVersion?.build ?? "..."}
            </Chip>
          </div>
        </div>

        <div className="grid md:grid-cols-2 gap-6">
          <Card className="h-fit">
            <CardHeader className="pb-2">
              <div className="flex items-center gap-2">
                <Info fill="#666" size={20} theme="outline" />
                <h2 className="text-xl font-semibold">功能特性</h2>
              </div>
            </CardHeader>
            <CardBody>
              <ul className="space-y-2">
                {features.map((feature, index) => (
                  <li key={index} className="flex items-center gap-2 text-gray-700 dark:text-gray-300">
                    <span className="w-1.5 h-1.5 rounded-full bg-blue-500 flex-shrink-0" />
                    {feature}
                  </li>
                ))}
              </ul>
            </CardBody>
          </Card>

          <Card className="h-fit">
            <CardHeader className="pb-2">
              <h2 className="text-xl font-semibold">项目链接</h2>
            </CardHeader>
            <CardBody>
              <div className="flex flex-col gap-3">
                <Link
                  isExternal
                  href="https://github.com/vst93/bili-fm"
                  color="primary"
                  className="flex items-center gap-2"
                >
                  <Github size={18} />
                  GitHub
                </Link>
                <Link
                  isExternal
                  href="https://gitee.com/vst93/bili-fm"
                  color="secondary"
                  className="flex items-center gap-2"
                >
                  Gitee
                </Link>
              </div>
            </CardBody>
          </Card>
        </div>

        <div className="text-center text-sm text-gray-500 dark:text-gray-400 pt-4">
          <Divider className="mb-4" />
          <p>
            本项目仅用于学习和研究。如果存在侵权，请联系我们删除。
          </p>
          <p className="mt-1">
            感谢{" "}
            <Link isExternal href="https://wails.io" size="sm" color="primary">
              Wails
            </Link>
            {" / "}
            <Link isExternal href="https://heroui.com" size="sm" color="primary">
              HeroUI
            </Link>
            {" / "}
            <Link
              isExternal
              href="https://github.com/SocialSisterYi/bilibili-API-collect"
              size="sm"
              color="primary"
            >
              bilibili-API-collect
            </Link>
            {" "}等开源项目
          </p>
        </div>
      </div>
    </DefaultLayout>
  );
}
