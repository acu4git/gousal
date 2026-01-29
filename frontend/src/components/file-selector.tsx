import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FieldLegend,
  FieldSet,
  FieldTitle,
} from "@/components/ui/field";
import { Button } from "@/components/ui/button";
import { Dispatch, SetStateAction, useState } from "react";
import { useLoading } from "@/context/loading/LoadingContext";

interface Props {
  mainList: string[];
  setMainFile: Dispatch<SetStateAction<string>>;
  trace: (file: string) => Promise<void>;
}

const FileSelector = ({ mainList, setMainFile, trace }: Props) => {
  const [selectedFile, setSelectedFile] = useState<string>("");
  const { show, hide } = useLoading();

  return (
    <div className="p-3">
      <FieldSet>
        <FieldLegend variant="label">{mainList.length} main files were found</FieldLegend>
        <FieldDescription>Select one to analyze</FieldDescription>
        <RadioGroup className="bg-stone-100 max-h-[70vh] overflow-y-scroll border border-black py-2 px-1">
          {mainList.map((f, idx) => (
            <FieldLabel
              htmlFor={f}
              key={idx}
              className="bg-white hover:cursor-pointer hover:bg-accent"
              onClick={() => {
                setSelectedFile(f);
              }}
            >
              <Field orientation="horizontal">
                <FieldContent>
                  <FieldTitle>{`(${idx + 1}) ${f}`}</FieldTitle>
                </FieldContent>
                <RadioGroupItem value={f} id={f} />
              </Field>
            </FieldLabel>
          ))}
        </RadioGroup>
      </FieldSet>
      <div className="flex justify-center">
        <Button
          className="my-5 hover:cursor-pointer"
          disabled={selectedFile === ""}
          onClick={async () => {
            show();
            try {
              await trace(selectedFile);
            } catch (e) {
              hide();
              return;
            }
            setMainFile(selectedFile);
            hide();
          }}
          variant="outline"
        >
          Select this
        </Button>
      </div>
    </div>
  );
};

export default FileSelector;
