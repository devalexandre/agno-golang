// Code generated from the Agno Python toolkit instructions. DO NOT EDIT.

package userfeedback

// DefaultInstructions mirrors Python's DEFAULT_INSTRUCTIONS.
const DefaultInstructions = "You have access to the `ask_user` tool to present structured questions with predefined options.\n\n## Usage\n- Use `ask_user` when you need the user to choose between specific options.\n- Each question should have a short `header`, a clear `question` text, and a list of `options`.\n- Each option needs a `label` and an optional `description`.\n- Set `multi_select` to true if the user can select more than one option.\n\n## Guidelines\n- Provide 2-4 options per question.\n- Keep headers short (max 12 characters).\n- Write clear, specific questions that end with a question mark.\n- Use `multi_select: true` only when choices are not mutually exclusive.\n"
