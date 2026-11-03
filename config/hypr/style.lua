local look = require("hyprtrixie.hypr.look")

local style = look.get("style")
if style ~= "soft" then
  style = "sharp"
end

require("hyprtrixie.hypr.style-" .. style)
