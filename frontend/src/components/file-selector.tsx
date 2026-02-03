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
import { Dispatch, SetStateAction, useEffect, useState } from "react";
import { useLoading } from "@/context/loading/LoadingContext";
import { Rel } from "#wailsjs/go/main/App";

interface Props {
  mainRecords: Record<string, string[]>;
  setMainFiles: Dispatch<SetStateAction<string[]>>;
  trace: (files: string[]) => Promise<void>;
}

const FileSelector = ({ mainRecords, setMainFiles, trace }: Props) => {
  const [selectedMain, setSelectedMain] = useState<{ dir: string; files: string[] } | null>(null);
  const [relMap, setRelMap] = useState<Record<string, string>>({});
  const { show, hide } = useLoading();

  useEffect(() => {
    const load = async () => {
      const entries = Object.entries(mainRecords);

      const pairs = await Promise.all(
        entries.flatMap(([dir, files]) =>
          files.map(async (file) => {
            const r = await Rel(dir, file);
            return [file, r] as const;
          })
        )
      );

      const map: Record<string, string> = {};
      pairs.forEach(([file, rel]) => {
        map[file] = rel;
      });

      setRelMap(map);
    };

    load();
  }, [mainRecords]);

  return (
    <div className="p-3">
      <FieldSet>
        <FieldLegend variant="label">
          {Object.keys(mainRecords).length} entry points were found
        </FieldLegend>
        <FieldDescription>Select one to analyze</FieldDescription>
        <RadioGroup className="bg-stone-100 max-h-[70vh] overflow-y-scroll border border-black py-2 px-1">
          {Object.entries(mainRecords).map(([dir, files], idx) => (
            <FieldLabel
              htmlFor={dir}
              key={idx}
              className="bg-white hover:cursor-pointer hover:bg-accent"
              onClick={() => {
                setSelectedMain({ dir: dir, files: files });
              }}
            >
              <Field orientation="horizontal">
                <FieldContent>
                  <FieldTitle>{`(${idx + 1}) ${dir}`}</FieldTitle>
                  <FieldDescription className="flex flex-col">
                    {files.map((file) => (
                      <p>{relMap[file]}</p>
                    ))}
                  </FieldDescription>
                </FieldContent>
                <RadioGroupItem value={dir} id={dir} />
              </Field>
            </FieldLabel>
          ))}
        </RadioGroup>
      </FieldSet>
      <div className="flex justify-center">
        <Button
          className="my-5 hover:cursor-pointer"
          disabled={!selectedMain}
          onClick={async () => {
            show();
            try {
              const m = selectedMain;
              if (!m) throw new Error("selectedMain is null");
              await trace(m.files);
              setMainFiles(m.files);
            } catch (e) {
              hide();
              return;
            }
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
