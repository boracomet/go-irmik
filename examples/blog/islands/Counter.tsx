import { useState } from "react";
import { createIsland } from "./_hydrate";

export type CounterProps = {
  initial?: number;
};

function Counter({ initial = 0 }: CounterProps) {
  const [count, setCount] = useState(initial);
  return (
    <button type="button" onClick={() => setCount((c) => c + 1)}>
      Count: {count}
    </button>
  );
}

export default Counter;

createIsland("Counter", Counter);
