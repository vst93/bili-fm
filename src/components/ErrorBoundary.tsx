import type { ErrorInfo, ReactNode } from "react";
import { Component } from "react";

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
}

/**
 * 全局渲染错误兜底。
 * 任一子树在渲染/生命周期中抛错时，React 会卸载整棵树并触发白屏；
 * 这里捕获后展示液态玻璃卡片，并提供「重试」重置 boundary 重新挂载子树。
 * 技术细节只进 console.error，不暴露给用户。
 */
class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false };

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("界面渲染出错:", error, info);
  }

  handleRetry = () => {
    this.setState({ hasError: false });
  };

  render() {
    if (!this.state.hasError) {
      return this.props.children;
    }

    return (
      <div className="error-boundary-overlay">
        <div className="error-boundary-card">
          <p className="error-boundary-title">界面出错了</p>
          <button
            className="error-boundary-retry"
            type="button"
            onClick={this.handleRetry}
          >
            重试
          </button>
        </div>
      </div>
    );
  }
}

export default ErrorBoundary;
