import type { FC } from "react";

/**
 * 抽屉列表首开时的加载骨架：6 行占位，结构与列表项一致
 * （圆图块 + 两行文本条），用 opacity 脉冲动画，不使用 blur/shadow 之外的合成层。
 */
const ListSkeleton: FC<{ rows?: number }> = ({ rows = 6 }) => {
  return (
    <div aria-hidden="true" className="list-skeleton-grid">
      {Array.from({ length: rows }).map((_, index) => (
        <div key={index} className="list-skeleton-item">
          <div className="list-skeleton-cover" />
          <div className="list-skeleton-line" />
          <div className="list-skeleton-line is-short" />
        </div>
      ))}
    </div>
  );
};

export default ListSkeleton;
