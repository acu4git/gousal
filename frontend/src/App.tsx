import { useState } from "react";
import logo from "@/assets/images/logo-universal.png";
import "@/App.css";
import { SelectGoProject } from "#wailsjs/go/main/App";

type Item = {
  id: number;
  name: string;
};

const App = () => {
  const [items, setItems] = useState<Item[]>([]);
  const [selectedId, setSelectedId] = useState<number>(-1);

  const selectGoProject = () => {
    SelectGoProject().then(updateItems);
  };

  const updateItems = (files: string[]) => {
    let newItems: Item[] = [];
    files.map((file, idx) => {
      const newcb: Item = {
        id: idx,
        name: file,
      };
      newItems.push(newcb);
    });
    setItems(newItems);
  };

  return (
    <div id="App">
      <img src={logo} id="logo" alt="logo" />
      <div className="flex flex-col items-baseline">
        {items.map((item) => (
          <label key={item.id}>
            <input
              type="radio"
              name="go-project-files"
              checked={item.id === selectedId}
              onChange={() => setSelectedId(item.id)}
            />
            {item.name}
          </label>
        ))}
      </div>
      <div id="input" className="input-box">
        <button className="hover:opacity-30" onClick={selectGoProject}>
          Select Go Project
        </button>
      </div>
    </div>
  );
};

export default App;
