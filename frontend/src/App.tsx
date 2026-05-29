import ToastContainer from "./components/toast/ToastContainer";
import TitleBar from "./components/titleBar";

import IndexPage from "@/pages/index";

function App() {
  return (
    <>
      <TitleBar />
      <IndexPage />
      <ToastContainer />
    </>
  );
}

export default App;
