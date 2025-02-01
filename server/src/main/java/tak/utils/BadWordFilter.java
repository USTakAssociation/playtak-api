package tak.utils;

import java.io.*;
import java.net.URL;
import java.util.HashMap;
import java.util.Map;

// based off of https://gist.github.com/PimDeWitte/c04cc17bc5fa9d7e3aee6670d4105941

public class BadWordFilter {

	static Map<String, String[]> words = new HashMap<>();

    static int largestWordLength = 0;

    public static void loadConfigs() {
        try {
            words.clear();
            BufferedReader reader = new BufferedReader(new InputStreamReader(new URL("https://docs.google.com/spreadsheets/d/1eVhQjZe1wbZdPQ4a2IlZ-jJUsgZmVoyrzXAjlQpA7hk/export?format=csv").openConnection().getInputStream()));
            String line = "";
            int counter = 0;
            while((line = reader.readLine()) != null) {
                counter++;
                String[] content = null;
                try {
                    content = line.split(",");
                    if(content.length == 0) {
                        continue;
                    }
                    String word = content[0];
                    String[] ignore_in_combination_with_words = new String[]{};
                    if(content.length > 1) {
                        ignore_in_combination_with_words = content[1].split("_");
                    }

                    if(word.length() > largestWordLength) {
                        largestWordLength = word.length();
                    }
                    words.put(word.replaceAll(" ", ""), ignore_in_combination_with_words);
                } catch(Exception e) {
                    e.printStackTrace();
                }

            }
            System.out.println("Loaded " + counter + " words to filter out");
        } catch (IOException e) {
            e.printStackTrace();
        }
    }

    private static final ThreadLocal<StringBuilder> sb = ThreadLocal.withInitial(StringBuilder::new);

    /**
     * Iterates over a String input and checks whether a cuss word was found in a list, then checks if the word should be ignored (e.g. bass contains the word *ss).
     * @param input string of text to check for bad words
     * @return array or strings
     */
	public static boolean badWordsFound(String input) {
		if (input == null) {
			return false;
		}
		input = input.toLowerCase();
		input = input.replaceAll("1","i");
        input = input.replaceAll("!","i");
        input = input.replaceAll("3","e");
        input = input.replaceAll("4","a");
        input = input.replaceAll("@","a");
        input = input.replaceAll("5","s");
        input = input.replaceAll("7","t");
        input = input.replaceAll("0","o");
        input = input.replaceAll("9","g");
		input = input.replaceAll("\\$","s");
		StringBuilder sb = BadWordFilter.sb.get();
		sb.setLength(0);
		for (int i = 0; i < input.length(); i++) {
			char c = input.charAt(i);
			sb.append(Character.toLowerCase(c));
		}

		// iterate over each letter in the word
		for (int start = 0; start < sb.length(); start++) {
			// from each letter, keep going to find bad words until either the end of the sentence is reached, or the max word length is reached.
			for (int offset = 1; offset < (sb.length() + 1 - start) && offset < largestWordLength; offset++) {
				//long hash = LongHashFunction.xx().hashChars(sb, start, offset);
                String wordToCheck = input.substring(start, start + offset);
				if (words.containsKey(wordToCheck)) {
					// for example, if you want to say the word bass, that should be possible.
					String[] ignoreCheck = words.get(wordToCheck);
					boolean ignore = false;
					for (int s = 0; s < ignoreCheck.length; s++) {
						if (indexOf(sb, ignoreCheck[s]) >= 0) {
							ignore = true;
							break;
						}
					}
					if (!ignore) {
						return true;
					}
				}
			}
		}
		
		return false;
	}
	
	private static int indexOf(CharSequence source, CharSequence target) {
		int sourceCount = source.length();
		int targetCount = target.length();
		int sourceOffset = 0;
		int targetOffset = 0;
		
		if (0 >= sourceCount) {
			return (targetCount == 0 ? sourceCount : -1);
		}
		if (targetCount == 0) {
			return 0;
		}
		
		char first = target.charAt(targetOffset);
		int max = sourceOffset + (sourceCount - targetCount);
		
		for (int i = sourceOffset; i <= max; i++) {
			/* Look for first character. */
			if (source.charAt(i) != first) {
				while (++i <= max && source.charAt(i) != first);
			}
			
			/* Found first character, now look at the rest of v2 */
			if (i <= max) {
				int j = i + 1;
				int end = j + targetCount - 1;
				for (int k = targetOffset + 1; j < end && source.charAt(j)
						== target.charAt(k); j++, k++);
				
				if (j == end) {
					/* Found whole string. */
					return i - sourceOffset;
				}
			}
		}
		return -1;
	}

   public static Boolean containsBadWord(String input) {
		// filter special characters
		input = input.replaceAll(" ", "");
	   	input = input.replaceAll("_", "");
		// split into words
		if(badWordsFound(input)) {
			return true;
		}
		return false;
   }

    public static String filterText(String input) {
		// loop over each word and if bad word, replace it with ****
		String[] words = input.split(" ");
		for(int i = 0; i < words.length; i++) {
			if(badWordsFound(words[i])) {
				words[i] = "****";
			}
		}
		// rebuild the string
		input = "";
		for(int i = 0; i < words.length; i++) {
			input += words[i] + " ";
		}
		// remove trailing space if there is one
		if(input.length() > 0 && input.charAt(input.length() - 1) == ' ') {
			input = input.substring(0, input.length() - 1);
		}
		return input;
    }
}
