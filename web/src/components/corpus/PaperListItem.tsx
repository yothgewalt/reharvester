import Chip from "@mui/material/Chip";
import ListItemButton from "@mui/material/ListItemButton";
import Typography from "@mui/material/Typography";

export interface PaperListItemProps {
  title: string;
  year: number;
  openAccess: boolean;
  selected: boolean;
  onSelect(): void;
}


export function PaperListItem({ title, year, openAccess, selected, onSelect }: PaperListItemProps) {
  return (
    <ListItemButton
      selected={selected}
      onClick={onSelect}
      sx={{ borderRadius: "6px", px: 1.5, py: 1 }}
    >
      <div className="flex min-w-0 flex-col gap-0.5">
        <Typography variant="body2" sx={{ fontWeight: 500 }} className="line-clamp-1">
          {title}
        </Typography>
        <div className="flex items-center gap-2">
          <Typography variant="caption" className="font-mono text-ink-2">
            {year}
          </Typography>
          {openAccess ? (
            <Chip
              label="OA"
              variant="outlined"
              size="small"
              sx={{ height: 20, fontSize: "0.6875rem" }}
            />
          ) : null}
        </div>
      </div>
    </ListItemButton>
  );
}
