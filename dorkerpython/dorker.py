#!/usr/bin/env python3
"""
Dorker - Google Dorking Search Tool
A simple CLI tool for performing Google dork searches and extracting first lines from results.
"""

import argparse
import sys
import os
import yaml
import requests
import re
from typing import List, Dict, Optional
from googleapiclient.discovery import build
from colorama import Fore, Style, init

# Initialize colorama for colored output
init(autoreset=True)


class DorkerConfig:
    """Handle configuration loading from YAML file."""
    
    def __init__(self, config_file: str = "config.yaml"):
        self.config_file = config_file
        self.credentials = []
        self.load_config()
    
    def load_config(self):
        """Load API keys and search engine IDs from config file."""
        if not os.path.exists(self.config_file):
            print(f"{Fore.RED}[ERROR] Config file not found: {self.config_file}")
            sys.exit(1)
        
        try:
            with open(self.config_file, 'r') as f:
                config = yaml.safe_load(f)
                self.credentials = config.get('google', [])
                print(f"{Fore.GREEN}[+] Loaded {len(self.credentials)} API credentials")
        except Exception as e:
            print(f"{Fore.RED}[ERROR] Failed to load config: {e}")
            sys.exit(1)
    
    def get_credentials(self, index: int = 0) -> Optional[Dict]:
        """Get credentials by index."""
        if index < len(self.credentials):
            return self.credentials[index]
        return None


class GoogleDorker:
    """Perform Google dork searches."""
    
    def __init__(self, config: DorkerConfig, verbose: bool = True):
        self.config = config
        self.verbose = verbose
        self.results = []
    
    def search(self, query: str, max_results: int = 10) -> List[Dict]:
        """
        Perform a Google search using the dork query.
        
        Args:
            query: The dork query to search for
            max_results: Maximum number of results to retrieve
        
        Returns:
            List of results with URL and first line content matching keywords
        """
        credentials = self.config.get_credentials(0)
        if not credentials:
            print(f"{Fore.RED}[ERROR] No credentials available")
            return []
        
        api_key = credentials.get('api_key')
        search_engine_id = credentials.get('search_engine_id')
        
        # Extract keywords from query (words that are not dork operators)
        keywords = self._extract_keywords(query)
        
        if self.verbose:
            print(f"{Fore.CYAN}[*] Starting search with query: {query}")
            print(f"{Fore.CYAN}[*] Keywords to match: {keywords}")
            print(f"{Fore.CYAN}[*] Using Search Engine ID: {search_engine_id[:10]}...")
        
        try:
            service = build("customsearch", "v1", developerKey=api_key)
            request = service.cse().list(q=query, cx=search_engine_id, num=min(10, max_results))
            response = request.execute()
            
            items = response.get('items', [])
            if self.verbose:
                print(f"{Fore.GREEN}[+] Found {len(items)} results")
            
            # Extract URLs and first lines matching keywords
            for item in items:
                url = item.get('link', '')
                title = item.get('title', '')
                snippet = item.get('snippet', '')
                
                result = {
                    'url': url,
                    'title': title,
                    'snippet': snippet,
                    'first_line': self._extract_first_line(url, snippet, keywords)
                }
                self.results.append(result)
                
                if self.verbose:
                    print(f"{Fore.YELLOW}[URL] {url}")
                    print(f"{Fore.WHITE}      Matching line: {result['first_line'][:100]}...")
            
            return self.results
        
        except Exception as e:
            print(f"{Fore.RED}[ERROR] Search failed: {e}")
            return []
    
    def _extract_keywords(self, query: str) -> List[str]:
        """
        Extract keywords from the dork query by removing dork operators.
        
        Args:
            query: The full dork query
        
        Returns:
            List of keywords to search for
        """
        # Remove common dork operators and their values
        operators = ['site:', 'ext:', 'inurl:', 'intitle:', 'intext:', 'filetype:', 'cache:', 'link:']
        
        # Split query and filter out operator parts
        keywords = []
        parts = query.split()
        
        for part in parts:
            # Skip if part starts with an operator
            is_operator = False
            for op in operators:
                if part.lower().startswith(op):
                    is_operator = True
                    break
            
            # If not an operator, treat it as a keyword
            if not is_operator and part.strip('"\''):
                keywords.append(part.strip('"\''))
        
        return keywords if keywords else ['']
    
    def _extract_first_line(self, url: str, snippet: str = "", keywords: List[str] = None) -> str:
        """
        Extract the first line that matches keywords from URL content.
        
        Args:
            url: The URL to fetch content from
            snippet: Fallback snippet if fetching fails
            keywords: List of keywords to search for in the content
        
        Returns:
            First line containing a keyword or first line of content
        """
        try:
            # Try to fetch the content from the URL
            headers = {'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)'}
            response = requests.get(url, headers=headers, timeout=5)
            response.raise_for_status()
            
            content = response.text
            
            # Try to decode PDF or binary content
            try:
                # Remove common PDF/binary junk characters
                content = re.sub(r'[\x00-\x08\x0B-\x0C\x0E-\x1F\x7F-\xFF]+', ' ', content)
                content = re.sub(r'\s+', ' ', content)  # Normalize whitespace
            except:
                pass
            
            # Split into lines
            lines = content.split('\n')
            
            # If keywords provided, find lines matching them
            if keywords:
                for line in lines:
                    cleaned = line.strip()
                    if cleaned and len(cleaned) > 5:
                        # Check if any keyword matches (case-insensitive)
                        for keyword in keywords:
                            if keyword.lower() in cleaned.lower():
                                # Clean up the line for display
                                cleaned = re.sub(r'\s+', ' ', cleaned)
                                return cleaned[:200]
            
            # Fallback: return first non-empty line with meaningful content
            for line in lines:
                cleaned = line.strip()
                # Skip PDF headers and binary data
                if cleaned and len(cleaned) > 10 and not cleaned.startswith('%PDF'):
                    # Clean up the line
                    cleaned = re.sub(r'\s+', ' ', cleaned)
                    return cleaned[:200]
            
            # Fallback to snippet
            return snippet[:150] if snippet else "No content extracted"
        
        except requests.RequestException as e:
            # Use snippet as fallback
            if self.verbose:
                print(f"{Fore.YELLOW}[!] Could not fetch content from {url}: {str(e)[:50]}")
            return snippet[:150] if snippet else "Failed to extract content"
        except Exception as e:
            return f"Error: {str(e)[:100]}"


class DorkerCLI:
    """Command-line interface for Dorker."""
    
    def __init__(self):
        self.config = DorkerConfig()
        self.parser = self._build_parser()
    
    def _build_parser(self) -> argparse.ArgumentParser:
        """Build the argument parser."""
        parser = argparse.ArgumentParser(
            description="Dorker - Google Dorking Search Tool",
            formatter_class=argparse.RawDescriptionHelpFormatter,
            epilog="""
Examples:
  dorker -q "site:roche.com ext:pdf confidential" -o results.txt
  dorker -q "inurl:admin" -o admin_pages.txt -v
  dorker -q "intitle:index.of" -o directory_listing.txt --max 20
            """
        )
        
        parser.add_argument(
            '-q', '--query',
            type=str,
            required=True,
            help='Google dork query to search for'
        )
        
        parser.add_argument(
            '-o', '--output',
            type=str,
            default='dorks.txt',
            help='Output file to save results (default: dorks.txt)'
        )
        
        parser.add_argument(
            '-v', '--verbose',
            action='store_true',
            default=True,
            help='Enable verbose output (enabled by default)'
        )
        
        parser.add_argument(
            '--quiet',
            action='store_true',
            help='Disable verbose output'
        )
        
        parser.add_argument(
            '--max',
            type=int,
            default=10,
            help='Maximum number of results to retrieve (default: 10)'
        )
        
        return parser
    
    def run(self, args=None):
        """Run the CLI tool."""
        parsed_args = self.parser.parse_args(args)
        
        # Handle verbose flag
        verbose = parsed_args.verbose and not parsed_args.quiet
        
        if verbose:
            print(f"{Fore.CYAN}{'='*60}")
            print(f"{Fore.CYAN}Dorker - Google Dorking Search Tool")
            print(f"{Fore.CYAN}{'='*60}")
        
        # Perform search
        dorker = GoogleDorker(self.config, verbose=verbose)
        results = dorker.search(parsed_args.query, max_results=parsed_args.max)
        
        if not results:
            print(f"{Fore.RED}[ERROR] No results found")
            return 1
        
        # Save results to file
        try:
            with open(parsed_args.output, 'w', encoding='utf-8') as f:
                for result in results:
                    f.write(f"URL: {result['url']}\n")
                    f.write(f"First Line: {result['first_line']}\n")
                    f.write(f"Snippet: {result['snippet']}\n")
                    f.write("-" * 80 + "\n\n")
            
            if verbose:
                print(f"\n{Fore.GREEN}[+] Results saved to: {parsed_args.output}")
                print(f"{Fore.GREEN}[+] Total results: {len(results)}")
                print(f"{Fore.CYAN}{'='*60}\n")
        
        except IOError as e:
            print(f"{Fore.RED}[ERROR] Failed to write to output file: {e}")
            return 1
        
        return 0


def main():
    """Main entry point."""
    cli = DorkerCLI()
    sys.exit(cli.run())


if __name__ == '__main__':
    main()
